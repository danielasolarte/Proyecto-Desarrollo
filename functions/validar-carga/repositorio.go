package validarcarga

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Acceso a Postgres. Son las mismas consultas de
// internal/media/postgres/media_repository.go (ClaimJob, TouchJobLock,
// UpdateJob, UpdateAsset, CreateJob) y de UpdateResource en el módulo de
// cursos, reducidas a las columnas que toca la validación. Se copian en vez
// de importarse porque esta carpeta es un módulo aparte: el backend pide Go
// 1.26 y el runtime de la función es go125.

const (
	tipoValidar = "validate_upload"
	tipoEscaneo = "scan_antivirus"
	maxIntentos = 3 // igual que defaultMaxAttempts en usecase/job_lifecycle.go
)

var (
	errNoExiste  = errors.New("el job no existe")
	errOtroTipo  = errors.New("el job no es validate_upload")
	errTerminado = errors.New("el job ya está done o dead_letter")
	errBloqueado = errors.New("otra entrega está procesando el job")
)

type job struct {
	ID          string
	AssetID     string
	Tipo        string
	Estado      string
	Intentos    int
	MaxIntentos int
}

type asset struct {
	ID         string
	ResourceID string
	Kind       string
	Clave      string  // original_storage_key
	Checksum   *string // el declarado por el cliente hasta que se valida
}

// repositorio es lo que la función necesita de Postgres. pgRepo lo
// implementa; las pruebas usan uno en memoria.
type repositorio interface {
	TomarJob(ctx context.Context, id string, ttl time.Duration) (*job, error)
	RenovarBloqueo(ctx context.Context, id string) error
	BuscarAsset(ctx context.Context, id string) (*asset, error)
	EstadoRecurso(ctx context.Context, resourceID, estado string) error
	GuardarValidacion(ctx context.Context, assetID, checksum, mime string) error
	// CrearJobSiNoExiste crea el job con esa clave de idempotencia o
	// devuelve el que ya existe (creado=false).
	CrearJobSiNoExiste(ctx context.Context, assetID, tipo, clave string) (j *job, creado bool, err error)
	CerrarJob(ctx context.Context, j *job) error
	// FallarJob deja el job pending (quedan intentos) o dead_letter.
	FallarJob(ctx context.Context, j *job, causa error) error
}

type pgRepo struct{ db *pgxpool.Pool }

const columnasJob = `id, media_asset_id, job_type, status, attempts, max_attempts`

func leerJob(row pgx.Row) (*job, error) {
	var j job
	err := row.Scan(&j.ID, &j.AssetID, &j.Tipo, &j.Estado, &j.Intentos, &j.MaxIntentos)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// TomarJob es el UPDATE atómico de ClaimJob, con una condición más: solo
// toma jobs validate_upload. Si un mensaje trae el ID de otro tipo de job,
// la función no lo toca (es del worker).
func (r *pgRepo) TomarJob(ctx context.Context, id string, ttl time.Duration) (*job, error) {
	j, err := leerJob(r.db.QueryRow(ctx, `
		UPDATE media_jobs SET
			status = 'processing', attempts = attempts + 1, locked_at = now(), updated_at = now()
		WHERE id = $1 AND job_type = $3 AND (
			status IN ('pending', 'failed')
			OR (status = 'processing' AND (locked_at IS NULL OR locked_at < now() - make_interval(secs => $2)))
		)
		RETURNING `+columnasJob, id, ttl.Seconds(), tipoValidar))
	if err == nil {
		return j, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	actual, err := leerJob(r.db.QueryRow(ctx, `SELECT `+columnasJob+` FROM media_jobs WHERE id = $1`, id))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, errNoExiste
	case err != nil:
		return nil, err
	case actual.Tipo != tipoValidar:
		return actual, errOtroTipo
	case actual.Estado == "done" || actual.Estado == "dead_letter":
		return actual, errTerminado
	default:
		return actual, errBloqueado
	}
}

func (r *pgRepo) RenovarBloqueo(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE media_jobs SET locked_at = now() WHERE id = $1 AND status = 'processing'`, id)
	return err
}

func (r *pgRepo) BuscarAsset(ctx context.Context, id string) (*asset, error) {
	var a asset
	err := r.db.QueryRow(ctx, `
		SELECT id, resource_id, kind, original_storage_key, checksum_sha256
		FROM media_assets WHERE id = $1`, id).
		Scan(&a.ID, &a.ResourceID, &a.Kind, &a.Clave, &a.Checksum)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// EstadoRecurso cambia resources.processing_status, lo mismo que
// setResourceProcessingStatus en el worker.
func (r *pgRepo) EstadoRecurso(ctx context.Context, resourceID, estado string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE resources SET processing_status = $1, updated_at = now() WHERE id = $2`, estado, resourceID)
	return err
}

// GuardarValidacion deja el checksum y el MIME reales en el asset, en lugar
// de los declarados por el cliente.
func (r *pgRepo) GuardarValidacion(ctx context.Context, assetID, checksum, mime string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE media_assets SET checksum_sha256 = $1, original_mime_type = $2, updated_at = now()
		WHERE id = $3`, checksum, mime, assetID)
	return err
}

// CrearJobSiNoExiste hace lo de usecase.EnqueueJob antes de publicar. Con
// ON CONFLICT dos entregas simultáneas no pueden crear dos jobs: la
// restricción UNIQUE (idempotency_key) de la 000004 decide.
func (r *pgRepo) CrearJobSiNoExiste(ctx context.Context, assetID, tipo, clave string) (*job, bool, error) {
	j, err := leerJob(r.db.QueryRow(ctx, `
		INSERT INTO media_jobs (media_asset_id, job_type, status, attempts, max_attempts, idempotency_key)
		VALUES ($1, $2, 'pending', 0, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING `+columnasJob, assetID, tipo, maxIntentos, clave))
	if err == nil {
		return j, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	j, err = leerJob(r.db.QueryRow(ctx,
		`SELECT `+columnasJob+` FROM media_jobs WHERE idempotency_key = $1`, clave))
	if err != nil {
		return nil, false, err
	}
	return j, false, nil
}

func (r *pgRepo) CerrarJob(ctx context.Context, j *job) error {
	_, err := r.db.Exec(ctx, `
		UPDATE media_jobs SET status = 'done', locked_at = NULL, last_error = NULL, updated_at = now()
		WHERE id = $1`, j.ID)
	if err == nil {
		j.Estado = "done"
	}
	return err
}

// FallarJob es usecase.MarkJobFailed: guarda el error y, si ya no quedan
// intentos, deja el job en dead_letter.
func (r *pgRepo) FallarJob(ctx context.Context, j *job, causa error) error {
	estado := "pending"
	if j.Intentos >= j.MaxIntentos {
		estado = "dead_letter"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE media_jobs SET status = $1, attempts = $2, last_error = $3, locked_at = NULL, updated_at = now()
		WHERE id = $4`, estado, j.Intentos, causa.Error(), j.ID)
	if err == nil {
		j.Estado = estado
	}
	return err
}
