package validarcarga

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// almacenPostgres usa las mismas consultas que el backend
// (internal/media/postgres/media_repository.go) para que la función y el
// worker se coordinen igual sobre media_jobs.
type almacenPostgres struct {
	db *pgxpool.Pool
}

const columnasTrabajo = `id, media_asset_id, job_type, status, attempts, max_attempts`

func leerTrabajo(row pgx.Row) (*trabajo, error) {
	var t trabajo
	if err := row.Scan(&t.ID, &t.AssetID, &t.Tipo, &t.Estado, &t.Intentos, &t.MaxIntentos); err != nil {
		return nil, err
	}
	return &t, nil
}

// TomarTrabajo es el UPDATE condicional de ClaimJob: de dos entregas
// simultáneas, solo una recibe la fila.
func (s *almacenPostgres) TomarTrabajo(ctx context.Context, id string, ttl time.Duration) (*trabajo, error) {
	t, err := leerTrabajo(s.db.QueryRow(ctx, `
		UPDATE media_jobs SET
			status = 'processing', attempts = attempts + 1, locked_at = now(), updated_at = now()
		WHERE id = $1 AND (
			status IN ('pending', 'failed')
			OR (status = 'processing' AND (locked_at IS NULL OR locked_at < now() - make_interval(secs => $2)))
		)
		RETURNING `+columnasTrabajo, id, ttl.Seconds()))
	if err == nil {
		return t, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	actual, err := leerTrabajo(s.db.QueryRow(ctx, `SELECT `+columnasTrabajo+` FROM media_jobs WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNoExiste
	}
	if err != nil {
		return nil, err
	}
	if actual.Estado == estadoTerminado || actual.Estado == estadoDeadLetter {
		return actual, errTrabajoTerminado
	}
	return actual, errTrabajoTomado
}

func (s *almacenPostgres) RenovarBloqueo(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `UPDATE media_jobs SET locked_at = now() WHERE id = $1 AND status = 'processing'`, id)
	return err
}

func (s *almacenPostgres) Activo(ctx context.Context, id string) (*activo, error) {
	var a activo
	err := s.db.QueryRow(ctx, `
		SELECT id, resource_id, kind, original_storage_key, checksum_sha256
		FROM media_assets WHERE id = $1`, id).
		Scan(&a.ID, &a.RecursoID, &a.Kind, &a.ClaveBlob, &a.Checksum)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNoExiste
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *almacenPostgres) GuardarValidacion(ctx context.Context, assetID, checksum, mime string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE media_assets SET checksum_sha256 = $1, original_mime_type = $2, updated_at = now()
		WHERE id = $3`, checksum, mime, assetID)
	return err
}

func (s *almacenPostgres) EstadoRecurso(ctx context.Context, recursoID, estado string) error {
	_, err := s.db.Exec(ctx, `UPDATE resources SET processing_status = $1, updated_at = now() WHERE id = $2`, estado, recursoID)
	return err
}

// CerrarTrabajo es MarkJobDone.
func (s *almacenPostgres) CerrarTrabajo(ctx context.Context, t *trabajo) error {
	t.Estado = estadoTerminado
	_, err := s.db.Exec(ctx, `
		UPDATE media_jobs SET status = 'done', last_error = NULL, locked_at = NULL, updated_at = now()
		WHERE id = $1`, t.ID)
	return err
}

// FallarTrabajo es MarkJobFailed: pending si quedan intentos, dead_letter
// si no.
func (s *almacenPostgres) FallarTrabajo(ctx context.Context, t *trabajo, causa string) error {
	t.Estado = estadoPendiente
	if t.Intentos >= t.MaxIntentos {
		t.Estado = estadoDeadLetter
	}
	_, err := s.db.Exec(ctx, `
		UPDATE media_jobs SET status = $1, attempts = $2, last_error = $3, locked_at = NULL, updated_at = now()
		WHERE id = $4`, t.Estado, t.Intentos, causa, t.ID)
	return err
}

// TrabajoSiguiente es EnqueueJob sin la publicación: crea el trabajo con
// su clave de idempotencia o encuentra el que ya existe. Solo se publica
// si quedó pending (si ya está en proceso o terminado, no se repite).
func (s *almacenPostgres) TrabajoSiguiente(ctx context.Context, assetID, tipo, clave string) (*trabajo, bool, error) {
	t, err := leerTrabajo(s.db.QueryRow(ctx, `
		INSERT INTO media_jobs (media_asset_id, job_type, status, attempts, max_attempts, idempotency_key)
		VALUES ($1, $2, 'pending', 0, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING `+columnasTrabajo, assetID, tipo, intentosPorDefecto, clave))
	if err == nil {
		return t, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	t, err = leerTrabajo(s.db.QueryRow(ctx, `SELECT `+columnasTrabajo+` FROM media_jobs WHERE idempotency_key = $1`, clave))
	if err != nil {
		return nil, false, err
	}
	return t, t.Estado == estadoPendiente, nil
}
