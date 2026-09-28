// Package postgres implementa domain.MediaRepository contra PostgreSQL
// usando pgx. Es la única capa del módulo que sabe escribir SQL. Calcado
// del repositorio del módulo de cursos: mismos helpers de nulabilidad,
// mismo wrapErr, mismo estilo de queries explícitas.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

type MediaRepository struct {
	db *pgxpool.Pool
}

func NewMediaRepository(db *pgxpool.Pool) *MediaRepository {
	return &MediaRepository{db: db}
}

// ---------- helpers de nulabilidad (mismo patrón que courses/postgres) ----------

func strPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func timePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	v := nt.Time
	return &v
}

func int64Ptr(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	v := ni.Int64
	return &v
}

func intPtr(ni sql.NullInt32) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int32)
	return &v
}

func nullableStr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullableTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func nullableInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func nullableInt(i *int) sql.NullInt32 {
	if i == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(*i), Valid: true}
}

func wrapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// rowScanner abstrae pgx.Row y pgx.Rows, que comparten el método Scan
// (mismo helper que en courses/postgres).
type rowScanner interface {
	Scan(dest ...any) error
}

// ---------- media_assets ----------

func (r *MediaRepository) CreateAsset(a *domain.MediaAsset) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO media_assets (
			resource_id, kind, original_storage_key, original_mime_type,
			original_size_bytes, checksum_sha256, hls_manifest_key,
			duration_seconds, converted_pdf_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`,
		a.ResourceID, a.Kind, a.OriginalStorageKey, nullableStr(a.OriginalMimeType),
		nullableInt64(a.OriginalSizeBytes), nullableStr(a.ChecksumSHA256), nullableStr(a.HLSManifestKey),
		nullableInt(a.DurationSeconds), nullableStr(a.ConvertedPDFKey),
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func scanAssetRow(row rowScanner) (*domain.MediaAsset, error) {
	var a domain.MediaAsset
	var mimeType, checksum, hlsKey, pdfKey sql.NullString
	var sizeBytes sql.NullInt64
	var duration sql.NullInt32
	err := row.Scan(
		&a.ID, &a.ResourceID, &a.Kind, &a.OriginalStorageKey, &mimeType,
		&sizeBytes, &checksum, &hlsKey, &duration, &pdfKey, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.OriginalMimeType = strPtr(mimeType)
	a.OriginalSizeBytes = int64Ptr(sizeBytes)
	a.ChecksumSHA256 = strPtr(checksum)
	a.HLSManifestKey = strPtr(hlsKey)
	a.DurationSeconds = intPtr(duration)
	a.ConvertedPDFKey = strPtr(pdfKey)
	return &a, nil
}

const selectAssetColumns = `
	id, resource_id, kind, original_storage_key, original_mime_type,
	original_size_bytes, checksum_sha256, hls_manifest_key,
	duration_seconds, converted_pdf_key, created_at, updated_at
`

func (r *MediaRepository) FindAssetByID(id string) (*domain.MediaAsset, error) {
	a, err := scanAssetRow(r.db.QueryRow(context.Background(),
		`SELECT `+selectAssetColumns+` FROM media_assets WHERE id = $1`, id))
	if err != nil {
		return nil, wrapErr(err)
	}
	return a, nil
}

func (r *MediaRepository) FindAssetByResourceID(resourceID string) (*domain.MediaAsset, error) {
	a, err := scanAssetRow(r.db.QueryRow(context.Background(),
		`SELECT `+selectAssetColumns+` FROM media_assets WHERE resource_id = $1`, resourceID))
	if err != nil {
		return nil, wrapErr(err)
	}
	return a, nil
}

func (r *MediaRepository) UpdateAsset(a *domain.MediaAsset) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE media_assets SET
			original_mime_type = $1, original_size_bytes = $2, checksum_sha256 = $3,
			hls_manifest_key = $4, duration_seconds = $5, converted_pdf_key = $6, updated_at = now()
		WHERE id = $7
	`,
		nullableStr(a.OriginalMimeType), nullableInt64(a.OriginalSizeBytes), nullableStr(a.ChecksumSHA256),
		nullableStr(a.HLSManifestKey), nullableInt(a.DurationSeconds), nullableStr(a.ConvertedPDFKey), a.ID,
	)
	return err
}

// ---------- upload_sessions ----------

func (r *MediaRepository) CreateUploadSession(s *domain.UploadSession) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO upload_sessions (
			resource_id, initiated_by, storage_key, storage_upload_id, status,
			expected_mime_type, expected_size_bytes, declared_checksum_sha256
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, expires_at, updated_at
	`,
		s.ResourceID, s.InitiatedBy, s.StorageKey, nullableStr(s.StorageUploadID), s.Status,
		nullableStr(s.ExpectedMimeType), nullableInt64(s.ExpectedSizeBytes), nullableStr(s.DeclaredChecksumSHA256),
	).Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &s.UpdatedAt)
}

func scanUploadSessionRow(row rowScanner) (*domain.UploadSession, error) {
	var s domain.UploadSession
	var storageUploadID, mimeType, checksum sql.NullString
	var sizeBytes sql.NullInt64
	var completedAt sql.NullTime
	err := row.Scan(
		&s.ID, &s.ResourceID, &s.InitiatedBy, &s.StorageKey, &storageUploadID, &s.Status,
		&mimeType, &sizeBytes, &checksum, &s.CreatedAt, &s.ExpiresAt, &completedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.StorageUploadID = strPtr(storageUploadID)
	s.ExpectedMimeType = strPtr(mimeType)
	s.ExpectedSizeBytes = int64Ptr(sizeBytes)
	s.DeclaredChecksumSHA256 = strPtr(checksum)
	s.CompletedAt = timePtr(completedAt)
	return &s, nil
}

const selectUploadSessionColumns = `
	id, resource_id, initiated_by, storage_key, storage_upload_id, status,
	expected_mime_type, expected_size_bytes, declared_checksum_sha256,
	created_at, expires_at, completed_at, updated_at
`

func (r *MediaRepository) FindUploadSessionByID(id string) (*domain.UploadSession, error) {
	s, err := scanUploadSessionRow(r.db.QueryRow(context.Background(),
		`SELECT `+selectUploadSessionColumns+` FROM upload_sessions WHERE id = $1`, id))
	if err != nil {
		return nil, wrapErr(err)
	}
	return s, nil
}

func (r *MediaRepository) UpdateUploadSession(s *domain.UploadSession) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE upload_sessions SET
			storage_upload_id = $1, status = $2, completed_at = $3, updated_at = now()
		WHERE id = $4
	`, nullableStr(s.StorageUploadID), s.Status, nullableTime(s.CompletedAt), s.ID)
	return err
}

// ExpireStaleUploadSessions marca como 'expired' toda sesión que sigue
// abierta ('initiated' o 'uploading') y superó su expires_at. Pensado
// para correrse periódicamente desde el worker (ver sección 5.1, punto 5:
// "reanudable durante 24 horas").
func (r *MediaRepository) ExpireStaleUploadSessions(olderThan time.Time) (int, error) {
	tag, err := r.db.Exec(context.Background(), `
		UPDATE upload_sessions
		SET status = 'expired', updated_at = now()
		WHERE status IN ('initiated', 'uploading') AND expires_at < $1
	`, olderThan)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ---------- media_jobs ----------

func (r *MediaRepository) CreateJob(j *domain.MediaJob) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO media_jobs (media_asset_id, job_type, status, attempts, max_attempts, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, j.MediaAssetID, j.JobType, j.Status, j.Attempts, j.MaxAttempts, j.IdempotencyKey).
		Scan(&j.ID, &j.CreatedAt, &j.UpdatedAt)
}

func scanJobRow(row rowScanner) (*domain.MediaJob, error) {
	var j domain.MediaJob
	var lastError sql.NullString
	var lockedAt sql.NullTime
	err := row.Scan(
		&j.ID, &j.MediaAssetID, &j.JobType, &j.Status, &j.Attempts, &j.MaxAttempts,
		&lastError, &j.IdempotencyKey, &lockedAt, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	j.LastError = strPtr(lastError)
	j.LockedAt = timePtr(lockedAt)
	return &j, nil
}

const selectJobColumns = `
	id, media_asset_id, job_type, status, attempts, max_attempts,
	last_error, idempotency_key, locked_at, created_at, updated_at
`

func (r *MediaRepository) FindJobByID(id string) (*domain.MediaJob, error) {
	j, err := scanJobRow(r.db.QueryRow(context.Background(),
		`SELECT `+selectJobColumns+` FROM media_jobs WHERE id = $1`, id))
	if err != nil {
		return nil, wrapErr(err)
	}
	return j, nil
}

// FindJobByIdempotencyKey es la pieza clave de la tolerancia a fallos
// (sección 6): antes de encolar, se busca si ya existe un job con la
// misma clave para no producir una salida duplicada.
func (r *MediaRepository) FindJobByIdempotencyKey(key string) (*domain.MediaJob, error) {
	j, err := scanJobRow(r.db.QueryRow(context.Background(),
		`SELECT `+selectJobColumns+` FROM media_jobs WHERE idempotency_key = $1`, key))
	if err != nil {
		return nil, wrapErr(err)
	}
	return j, nil
}

func (r *MediaRepository) UpdateJob(j *domain.MediaJob) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE media_jobs SET
			status = $1, attempts = $2, last_error = $3, locked_at = $4, updated_at = now()
		WHERE id = $5
	`, j.Status, j.Attempts, nullableStr(j.LastError), nullableTime(j.LockedAt), j.ID)
	return err
}

func (r *MediaRepository) ListDeadLetterJobs() ([]domain.MediaJob, error) {
	rows, err := r.db.Query(context.Background(),
		`SELECT `+selectJobColumns+` FROM media_jobs WHERE status = 'dead_letter' ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.MediaJob
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}
