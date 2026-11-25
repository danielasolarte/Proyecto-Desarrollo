// Package validarcarga es la Cloud Function mooc-validar-carga (Entrega 5).
//
// Recibe validate_upload desde el topic mooc-media-validate (Eventarc),
// valida el original en Cloud Storage (SHA-256 real, MIME real y firma ZIP
// de las presentaciones) y, si pasa, encola scan_antivirus para el worker.
// Reemplaza la validación que hacía el worker al empezar scan_antivirus.
//
// Contrato, idempotencia y reintentos: docs/entrega5/contrato-validacion.md.
// Devolver nil confirma el mensaje; devolver error hace que Eventarc lo
// reintente con espera creciente.
package validarcarga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// Igual que el worker (cmd/worker): un bloqueo de más de 60 s sin
	// renovar se considera de una instancia muerta.
	ttlBloqueo = 60 * time.Second
	latido     = 20 * time.Second
)

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: formatoCloudLogging})))
	functions.CloudEvent("ValidarCarga", ValidarCarga)
}

// formatoCloudLogging renombra level y msg a severity y message, que es lo
// que Cloud Logging lee de un registro JSON.
func formatoCloudLogging(grupos []string, a slog.Attr) slog.Attr {
	if len(grupos) > 0 {
		return a
	}
	switch a.Key {
	case slog.LevelKey:
		nivel := a.Value.Any().(slog.Level)
		sev := "INFO"
		switch {
		case nivel >= slog.LevelError:
			sev = "ERROR"
		case nivel >= slog.LevelWarn:
			sev = "WARNING"
		case nivel < slog.LevelInfo:
			sev = "DEBUG"
		}
		return slog.String("severity", sev)
	case slog.MessageKey:
		return slog.String("message", a.Value.String())
	}
	return a
}

// mensajePubSub es el cuerpo del CloudEvent
// google.cloud.pubsub.topic.v1.messagePublished.
type mensajePubSub struct {
	Message struct {
		Data       []byte            `json:"data"`
		Attributes map[string]string `json:"attributes"`
		MessageID  string            `json:"messageId"`
	} `json:"message"`
}

type mensajeTrabajo struct {
	JobID string `json:"job_id"`
}

// ValidarCarga es el punto de entrada (entry_point en funcion.tf).
func ValidarCarga(ctx context.Context, e event.Event) error {
	var m mensajePubSub
	if err := e.DataAs(&m); err != nil {
		slog.Error("evento inválido, se descarta", "error", err.Error(), "estado_final", "descartado")
		return nil
	}
	var t mensajeTrabajo
	if err := json.Unmarshal(m.Message.Data, &t); err != nil || t.JobID == "" {
		slog.Error("mensaje sin job_id, se descarta", "message_id", m.Message.MessageID, "estado_final", "descartado")
		return nil
	}
	log := slog.With("job_id", t.JobID, "message_id", m.Message.MessageID)

	v, err := dependencias(ctx)
	if err != nil {
		log.Error("no se pudo iniciar la función, Eventarc reintentará", "error", err.Error(), "estado_final", "reintento")
		return err
	}
	return v.procesar(ctx, t.JobID, log)
}

// ---------- dependencias ----------

var (
	mu         sync.Mutex
	compartido *validador
)

// dependencias crea los clientes la primera vez y los reutiliza en las
// siguientes invocaciones de la misma instancia. Si falla, la próxima
// invocación lo vuelve a intentar.
func dependencias(ctx context.Context) (*validador, error) {
	mu.Lock()
	defer mu.Unlock()
	if compartido != nil {
		return compartido, nil
	}
	cfg := map[string]string{}
	for _, k := range []string{"DATABASE_URL", "S3_BUCKET", "PUBSUB_PROJECT", "PUBSUB_TOPIC"} {
		cfg[k] = os.Getenv(k)
		if cfg[k] == "" {
			return nil, fmt.Errorf("falta la variable %s", k)
		}
	}

	pool, err := pgxpool.ParseConfig(cfg["DATABASE_URL"])
	if err != nil {
		return nil, errors.New("DATABASE_URL no es válida") // sin repetir la URL: lleva la clave
	}
	pool.MaxConns = 2
	if n, err := strconv.Atoi(os.Getenv("DB_MAX_CONNS")); err == nil && n > 0 {
		pool.MaxConns = int32(n)
	}
	db, err := pgxpool.NewWithConfig(context.Background(), pool)
	if err != nil {
		return nil, fmt.Errorf("conectando a Postgres: %w", err)
	}

	// Sin Client.Timeout: cortaría la lectura en flujo de un video grande.
	// El límite total lo pone el contexto de la invocación (timeout_s).
	cli := &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}}
	tk := &metadatos{cli: &http.Client{Timeout: 3 * time.Second}}

	compartido = &validador{
		repo: &pgRepo{db: db},
		almacen: gcsAlmacen{
			base: "https://storage.googleapis.com", bucket: cfg["S3_BUCKET"], cli: cli, tk: tk,
		},
		cola: pubsubCola{
			url: fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/topics/%s:publish",
				url.PathEscape(cfg["PUBSUB_PROJECT"]), url.PathEscape(cfg["PUBSUB_TOPIC"])),
			cli: cli, tk: tk,
		},
		ttl:    ttlBloqueo,
		latido: latido,
	}
	return compartido, nil
}

// ---------- el trabajo ----------

type validador struct {
	repo    repositorio
	almacen almacen
	cola    cola
	ttl     time.Duration
	latido  time.Duration
}

// procesar sigue la sección 2 del contrato. Devuelve nil cuando el mensaje
// se debe confirmar y error cuando Eventarc debe reintentarlo.
func (v *validador) procesar(ctx context.Context, jobID string, log *slog.Logger) error {
	inicio := time.Now()

	j, err := v.repo.TomarJob(ctx, jobID, v.ttl)
	switch {
	case errors.Is(err, errNoExiste):
		log.Error("el job no existe, se descarta", "estado_final", "descartado")
		return nil
	case errors.Is(err, errOtroTipo):
		log.Error("el job no es validate_upload, no se toca", "job_type", j.Tipo, "estado_final", "descartado")
		return nil
	case errors.Is(err, errTerminado):
		log.Info("reentrega de un job ya terminado, no se procesa", "status", j.Estado, "estado_final", "ya_terminado")
		return nil
	case errors.Is(err, errBloqueado):
		log.Warn("otra entrega está procesando el job, Eventarc reintentará", "status", j.Estado, "estado_final", "ocupado")
		return err
	case err != nil:
		log.Error("no se pudo tomar el job, Eventarc reintentará", "error", err.Error(), "estado_final", "reintento")
		return err
	}
	log = log.With("intento", j.Intentos, "max_intentos", j.MaxIntentos)
	log.Info("job tomado, validando")

	parar := v.mantenerBloqueo(j.ID, log)
	defer parar()

	a, err := v.repo.BuscarAsset(ctx, j.AssetID)
	if err != nil {
		return v.fallo(ctx, j, nil, fmt.Errorf("buscando el asset: %w", err), log, inicio)
	}
	log = log.With("asset_id", a.ID, "kind", a.Kind)

	if err := v.repo.EstadoRecurso(ctx, a.ResourceID, "processing"); err != nil {
		return v.fallo(ctx, j, a, fmt.Errorf("marcando el recurso en processing: %w", err), log, inicio)
	}

	r, err := v.almacen.Abrir(ctx, a.Clave)
	if err != nil {
		return v.fallo(ctx, j, a, fmt.Errorf("abriendo el original: %w", err), log, inicio)
	}
	in, err := inspeccionar(r)
	r.Close()
	if err != nil {
		return v.fallo(ctx, j, a, fmt.Errorf("leyendo el original: %w", err), log, inicio)
	}

	if motivo := revisar(a.Kind, a.Checksum, in); motivo != nil {
		// Fallo permanente: se borra el original y el job se va directo a
		// dead_letter, como hacía el worker.
		if err := v.almacen.Borrar(ctx, a.Clave); err != nil {
			log.Warn("no se pudo borrar el original rechazado", "error", err.Error())
		}
		j.Intentos = j.MaxIntentos
		return v.fallo(ctx, j, a, motivo, log, inicio)
	}

	if err := v.repo.GuardarValidacion(ctx, a.ID, in.Checksum, in.Mime); err != nil {
		return v.fallo(ctx, j, a, fmt.Errorf("guardando checksum y MIME: %w", err), log, inicio)
	}
	// Primero se encola el siguiente paso y después se cierra el job: si
	// algo falla entre los dos, el reintento vuelve a validar y encolar es
	// idempotente. Al revés, un job done sin scan_antivirus dejaría la
	// carga detenida.
	escaneo, err := v.encolarEscaneo(ctx, a.ID)
	if err != nil {
		return v.fallo(ctx, j, a, fmt.Errorf("encolando scan_antivirus: %w", err), log, inicio)
	}
	if err := v.repo.CerrarJob(ctx, j); err != nil {
		return v.fallo(ctx, j, a, fmt.Errorf("cerrando el job: %w", err), log, inicio)
	}

	log.Info("carga validada, scan_antivirus encolado",
		"checksum", in.Checksum, "mime", in.Mime, "scan_job_id", escaneo,
		"estado_final", "done", "duracion_ms", time.Since(inicio).Milliseconds())
	return nil
}

// encolarEscaneo es usecase.EnqueueJob para scan_antivirus: crea el job con
// la clave "<asset_id>:scan_antivirus" si no existe y lo publica si sigue
// pending. Si ya lo tomó el worker o terminó, no publica otra vez.
func (v *validador) encolarEscaneo(ctx context.Context, assetID string) (string, error) {
	clave := assetID + ":" + tipoEscaneo
	j, creado, err := v.repo.CrearJobSiNoExiste(ctx, assetID, tipoEscaneo, clave)
	if err != nil {
		return "", err
	}
	if creado || j.Estado == "pending" {
		if err := v.cola.Publicar(ctx, j.ID, tipoEscaneo); err != nil {
			return "", err
		}
	}
	return j.ID, nil
}

// fallo es MarkJobFailed más la regla de respuesta: con intentos
// disponibles, el job vuelve a pending y se devuelve error para que
// Eventarc reintente; sin intentos, queda en dead_letter, el recurso en
// failed y se confirma.
func (v *validador) fallo(ctx context.Context, j *job, a *asset, causa error, log *slog.Logger, inicio time.Time) error {
	if err := v.repo.FallarJob(ctx, j, causa); err != nil {
		log.Error("no se pudo registrar el fallo, Eventarc reintentará",
			"error", causa.Error(), "error_bd", err.Error(), "estado_final", "reintento")
		return causa
	}
	dur := time.Since(inicio).Milliseconds()
	if j.Estado != "dead_letter" {
		log.Warn("validación falló, Eventarc reintentará", "error", causa.Error(),
			"estado_final", "reintento", "duracion_ms", dur)
		return causa
	}
	if a != nil {
		if err := v.repo.EstadoRecurso(ctx, a.ResourceID, "failed"); err != nil {
			log.Warn("no se pudo marcar el recurso en failed", "error", err.Error())
		}
	}
	log.Error("job sin intentos, queda en dead_letter", "error", causa.Error(),
		"rechazada", errors.Is(causa, errRechazo), "estado_final", "dead_letter", "duracion_ms", dur)
	return nil
}

// mantenerBloqueo renueva locked_at mientras se valida, como el latido del
// worker (push.go), para que un archivo grande no pierda el bloqueo.
func (v *validador) mantenerBloqueo(jobID string, log *slog.Logger) func() {
	if v.latido <= 0 {
		return func() {}
	}
	listo := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(v.latido)
		defer t.Stop()
		for {
			select {
			case <-listo:
				return
			case <-t.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := v.repo.RenovarBloqueo(ctx, jobID); err != nil {
					log.Warn("no se pudo renovar el bloqueo del job", "error", err.Error())
				}
				cancel()
			}
		}
	}()
	return func() {
		close(listo)
		wg.Wait()
	}
}
