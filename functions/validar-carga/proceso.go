package validarcarga

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// Tipos y estados: los mismos valores que usa el backend en Postgres
// (internal/media/domain).
const (
	tipoValidar   = "validate_upload"
	tipoAntivirus = "scan_antivirus"

	estadoPendiente    = "pending"
	estadoEnProceso    = "processing"
	estadoTerminado    = "done"
	estadoDeadLetter   = "dead_letter"
	recursoEnProceso   = "processing"
	recursoFallido     = "failed"
	bloqueoTrabajo     = 60 * time.Second
	latidoBloqueo      = 20 * time.Second
	intentosPorDefecto = 3
)

// Errores que devuelve el almacén al tomar un trabajo (ClaimJob).
var (
	errTrabajoTomado    = errors.New("otro proceso tiene el trabajo")
	errTrabajoTerminado = errors.New("el trabajo ya terminó")
	errNoExiste         = errors.New("no existe")
)

type trabajo struct {
	ID          string
	AssetID     string
	Tipo        string
	Estado      string
	Intentos    int
	MaxIntentos int
}

type activo struct {
	ID        string
	RecursoID string
	Kind      string
	ClaveBlob string
	Checksum  *string
}

// almacen es lo que la función necesita de Postgres.
type almacen interface {
	TomarTrabajo(ctx context.Context, id string, ttl time.Duration) (*trabajo, error)
	RenovarBloqueo(ctx context.Context, id string) error
	Activo(ctx context.Context, id string) (*activo, error)
	GuardarValidacion(ctx context.Context, assetID, checksum, mime string) error
	EstadoRecurso(ctx context.Context, recursoID, estado string) error
	CerrarTrabajo(ctx context.Context, t *trabajo) error
	FallarTrabajo(ctx context.Context, t *trabajo, causa string) error
	// TrabajoSiguiente crea (o encuentra, por la clave de idempotencia) el
	// trabajo y dice si hay que publicarlo (si quedó pending).
	TrabajoSiguiente(ctx context.Context, assetID, tipo, clave string) (*trabajo, bool, error)
}

// archivos es lo que la función necesita del bucket.
type archivos interface {
	Abrir(ctx context.Context, clave string) (io.ReadCloser, error)
	Borrar(ctx context.Context, clave string) error
}

// publicador publica un trabajo en mooc-media-jobs con el contrato de la E4.
type publicador interface {
	Publicar(ctx context.Context, jobID, tipo string) error
}

type validador struct {
	db     almacen
	blobs  archivos
	cola   publicador
	log    *slog.Logger
	latido time.Duration
}

// procesar valida un trabajo validate_upload. Devolver nil confirma el
// mensaje; devolver error hace que Eventarc lo reintente.
func (v *validador) procesar(ctx context.Context, jobID string) error {
	log := v.log.With("job_id", jobID)

	t, err := v.db.TomarTrabajo(ctx, jobID, bloqueoTrabajo)
	switch {
	case errors.Is(err, errTrabajoTerminado):
		log.Info("mensaje repetido: el trabajo ya terminó, se confirma sin repetir nada")
		return nil
	case errors.Is(err, errNoExiste):
		log.Error("el trabajo no existe, se descarta el mensaje")
		return nil
	case errors.Is(err, errTrabajoTomado):
		log.Warn("otra instancia tiene el trabajo; Eventarc lo reintentará")
		return err
	case err != nil:
		return fmt.Errorf("tomando el trabajo: %w", err)
	}
	log = log.With("intento", t.Intentos, "max_intentos", t.MaxIntentos, "asset_id", t.AssetID)

	if t.Tipo != tipoValidar {
		t.Intentos = t.MaxIntentos
		msg := "la función solo ejecuta validate_upload, llegó " + t.Tipo
		if err := v.db.FallarTrabajo(ctx, t, msg); err != nil {
			return err
		}
		log.Error(msg, "estado_final", estadoDeadLetter)
		return nil
	}

	// Mientras se lee el archivo, se renueva el bloqueo para que otra
	// entrega no tome el mismo trabajo.
	ctxLatido, pararLatido := context.WithCancel(ctx)
	defer pararLatido()
	go v.mantenerBloqueo(ctxLatido, t.ID)

	a, err := v.db.Activo(ctx, t.AssetID)
	if err != nil {
		return v.falloTransitorio(ctx, log, t, nil, fmt.Errorf("leyendo el asset: %w", err))
	}
	log = log.With("recurso_id", a.RecursoID, "kind", a.Kind)

	if err := v.db.EstadoRecurso(ctx, a.RecursoID, recursoEnProceso); err != nil {
		return v.falloTransitorio(ctx, log, t, a, fmt.Errorf("marcando el recurso en proceso: %w", err))
	}

	ins, err := v.leer(ctx, a.ClaveBlob)
	if err != nil {
		return v.falloTransitorio(ctx, log, t, a, fmt.Errorf("leyendo el original: %w", err))
	}

	if motivo := motivoRechazo(a.Kind, a.Checksum, ins); motivo != "" {
		// Fallo permanente: reintentar no lo arregla.
		if err := v.blobs.Borrar(ctx, a.ClaveBlob); err != nil {
			log.Warn("no se pudo borrar el original rechazado", "error", err)
		}
		t.Intentos = t.MaxIntentos
		if err := v.db.FallarTrabajo(ctx, t, motivo); err != nil {
			return err
		}
		if err := v.db.EstadoRecurso(ctx, a.RecursoID, recursoFallido); err != nil {
			return err
		}
		log.Warn("archivo rechazado", "motivo", motivo, "mime", ins.MIME, "bytes", ins.Bytes, "estado_final", estadoDeadLetter)
		return nil
	}

	if err := v.db.GuardarValidacion(ctx, a.ID, ins.Checksum, ins.MIME); err != nil {
		return v.falloTransitorio(ctx, log, t, a, fmt.Errorf("guardando checksum y MIME: %w", err))
	}

	// Primero se encola el siguiente paso y después se cierra el trabajo:
	// si algo falla entre los dos, el reintento encuentra el antivirus ya
	// creado por su clave y no lo duplica.
	clave := a.ID + ":" + tipoAntivirus
	sig, publicar, err := v.db.TrabajoSiguiente(ctx, a.ID, tipoAntivirus, clave)
	if err != nil {
		return v.falloTransitorio(ctx, log, t, a, fmt.Errorf("creando scan_antivirus: %w", err))
	}
	if publicar {
		if err := v.cola.Publicar(ctx, sig.ID, tipoAntivirus); err != nil {
			return v.falloTransitorio(ctx, log, t, a, fmt.Errorf("publicando scan_antivirus: %w", err))
		}
	}
	if err := v.db.CerrarTrabajo(ctx, t); err != nil {
		return fmt.Errorf("cerrando el trabajo: %w", err)
	}
	log.Info("archivo validado", "checksum", ins.Checksum, "mime", ins.MIME, "bytes", ins.Bytes,
		"siguiente_job_id", sig.ID, "estado_final", estadoTerminado)
	return nil
}

func (v *validador) leer(ctx context.Context, clave string) (inspeccion, error) {
	r, err := v.blobs.Abrir(ctx, clave)
	if err != nil {
		return inspeccion{}, err
	}
	defer r.Close()
	return inspeccionar(r)
}

// falloTransitorio deja el trabajo para otro intento (y pide el reintento
// a Eventarc) o, si ya no quedan, en dead_letter con el recurso fallido.
func (v *validador) falloTransitorio(ctx context.Context, log *slog.Logger, t *trabajo, a *activo, causa error) error {
	if err := v.db.FallarTrabajo(ctx, t, causa.Error()); err != nil {
		return errors.Join(causa, err)
	}
	if t.Intentos >= t.MaxIntentos {
		if a != nil {
			if err := v.db.EstadoRecurso(ctx, a.RecursoID, recursoFallido); err != nil {
				log.Warn("no se pudo marcar el recurso como fallido", "error", err)
			}
		}
		log.Error("se agotaron los intentos", "error", causa, "estado_final", estadoDeadLetter)
		return nil
	}
	log.Warn("error transitorio, se reintenta", "error", causa)
	return causa
}

func (v *validador) mantenerBloqueo(ctx context.Context, id string) {
	cada := v.latido
	if cada <= 0 {
		cada = latidoBloqueo
	}
	tick := time.NewTicker(cada)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := v.db.RenovarBloqueo(ctx, id); err != nil && ctx.Err() == nil {
				v.log.Warn("no se pudo renovar el bloqueo", "job_id", id, "error", err)
			}
		}
	}
}
