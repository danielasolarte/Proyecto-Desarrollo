package worker

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
	mediaplatform "github.com/equipo-mooc/plataforma-mooc/internal/media/platform"
)

// maxPushBody limita el sobre de Pub/Sub; el mensaje real solo trae un ID.
const maxPushBody = 1 << 20

// JobRunner procesa un job ya tomado. *Processor lo implementa.
type JobRunner interface {
	HandleJob(job *mediadomain.MediaJob) error
}

// PushHandler atiende POST /pubsub/push (Entrega 4). Pub/Sub da el mensaje
// por confirmado con cualquier 2xx y lo reintenta con espera creciente con
// cualquier otra respuesta. Las reglas (docs/entrega4/contrato-mensaje.md):
//
//   - mensaje inválido o job que no existe: 204 (reintentar no lo arregla)
//   - job done o dead_letter: 204 sin procesar (es una reentrega)
//   - job processing con bloqueo vigente: 429 (otra entrega lo tiene)
//   - procesado y guardado: 204, solo después de guardar derivados y estado
//   - error con intentos disponibles: 500
//   - error sin intentos: el job queda dead_letter y se responde 204
//
// Mientras procesa renueva locked_at cada Heartbeat. Si la instancia muere,
// el bloqueo caduca a los LockTTL y la siguiente entrega retoma el job.
type PushHandler struct {
	Repo      mediadomain.MediaRepository
	Runner    JobRunner
	Logger    *slog.Logger
	LockTTL   time.Duration
	Heartbeat time.Duration
}

func (h *PushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPushBody))
	if err != nil {
		h.Logger.Warn("no se pudo leer el cuerpo del push", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	msg, env, err := mediaplatform.DecodePushEnvelope(body)
	log := h.Logger.With(
		"message_id", env.Message.MessageID,
		"delivery_attempt", env.DeliveryAttempt,
	)
	if err == nil {
		if _, perr := uuid.Parse(msg.JobID); perr != nil {
			err = errors.Join(mediaplatform.ErrInvalidMessage, perr)
		}
	}
	if err != nil {
		log.Error("mensaje inválido, se descarta", "error", err.Error(), "estado_final", "descartado")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	log = log.With("job_id", msg.JobID)

	job, err := h.Repo.ClaimJob(msg.JobID, h.LockTTL)
	switch {
	case errors.Is(err, mediadomain.ErrNotFound):
		log.Error("el job no existe, se descarta", "estado_final", "descartado")
		w.WriteHeader(http.StatusNoContent)
		return
	case errors.Is(err, mediadomain.ErrJobAlreadyTerminal):
		log.Info("reentrega de un job ya terminado, no se procesa",
			"status", string(job.Status), "estado_final", "ya_terminado")
		w.WriteHeader(http.StatusNoContent)
		return
	case errors.Is(err, mediadomain.ErrJobLocked):
		log.Warn("otra entrega está procesando el job, Pub/Sub reintentará",
			"status", string(job.Status), "estado_final", "ocupado")
		http.Error(w, "job en proceso", http.StatusTooManyRequests)
		return
	case err != nil:
		log.Error("no se pudo tomar el job", "error", err.Error(), "estado_final", "reintento")
		http.Error(w, "error tomando el job", http.StatusInternalServerError)
		return
	}

	log = log.With("job_type", string(job.JobType), "intento", job.Attempts, "max_intentos", job.MaxAttempts)
	log.Info("job tomado, procesando")

	stop := h.latido(job.ID, log)
	err = h.Runner.HandleJob(job)
	stop()

	durMs := time.Since(start).Milliseconds()
	switch {
	case err == nil:
		log.Info("job terminado y guardado", "estado_final", "done", "duracion_ms", durMs)
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrDeadLettered):
		log.Error("job sin intentos, queda en dead_letter", "error", err.Error(),
			"estado_final", "dead_letter", "duracion_ms", durMs)
		w.WriteHeader(http.StatusNoContent)
	default:
		log.Warn("job falló, Pub/Sub reintentará", "error", err.Error(),
			"estado_final", "reintento", "duracion_ms", durMs)
		http.Error(w, "job falló", http.StatusInternalServerError)
	}
}

// latido renueva locked_at cada h.Heartbeat hasta que se llame la función
// que devuelve, que espera a que el latido termine.
func (h *PushHandler) latido(jobID string, log *slog.Logger) func() {
	if h.Heartbeat <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(h.Heartbeat)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if err := h.Repo.TouchJobLock(jobID); err != nil {
					log.Warn("no se pudo renovar el bloqueo del job", "error", err.Error())
				}
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}
