-- Módulo: Multimedia + Workers (Persona 3)
--
-- IMPORTANTE PARA EL EQUIPO: si al momento de hacer merge ya existe una
-- migración 000004_algo.up.sql de otra persona, hay que renombrar este
-- archivo (y su .down.sql) al siguiente número libre y avisar en el
-- grupo, tal como quedó documentado en 000001_create_courses_catalog_tables.up.sql.
--
-- Esta migración agrega la contraparte de almacenamiento/procesamiento
-- para los recursos de tipo image, video, audio, pdf, presentation y file
-- creados por el módulo de cursos (resources.processing_status y
-- resources.media_asset_id ya existen desde la migración 000001 y están
-- pensados para llenarse desde aquí).

CREATE TABLE media_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    resource_id UUID NOT NULL UNIQUE
        REFERENCES resources(id) ON DELETE CASCADE,

    kind TEXT NOT NULL CHECK (kind IN (
        'image', 'video', 'audio', 'pdf', 'presentation', 'file'
    )),

    original_storage_key TEXT NOT NULL,
    original_mime_type    TEXT,
    original_size_bytes   BIGINT
        CHECK (original_size_bytes IS NULL OR original_size_bytes >= 0),
    checksum_sha256        TEXT,

    -- Solo aplica a video/audio: ruta del manifest .m3u8 en el bucket
    -- y duración detectada por ffprobe.
    hls_manifest_key   TEXT,
    duration_seconds   INT
        CHECK (duration_seconds IS NULL OR duration_seconds >= 0),

    -- Solo aplica a presentation (pptx/odp) convertidas a pdf.
    converted_pdf_key  TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE upload_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    resource_id UUID NOT NULL
        REFERENCES resources(id) ON DELETE CASCADE,

    initiated_by UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    storage_key TEXT NOT NULL,

    -- Id de la carga multipart en MinIO/S3 (UploadCreateMultipartUpload).
    -- Se necesita para reanudar o abortar la carga.
    storage_upload_id TEXT,

    status TEXT NOT NULL DEFAULT 'initiated'
        CHECK (status IN (
            'initiated', 'uploading', 'completed', 'aborted', 'expired'
        )),

    expected_mime_type  TEXT,
    expected_size_bytes BIGINT
        CHECK (expected_size_bytes IS NULL OR expected_size_bytes >= 0),

    -- Checksum que el cliente declara al iniciar; se contrasta contra el
    -- checksum real calculado al completar la carga.
    declared_checksum_sha256 TEXT,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '24 hours'),
    completed_at TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE media_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    media_asset_id UUID NOT NULL
        REFERENCES media_assets(id) ON DELETE CASCADE,

    job_type TEXT NOT NULL CHECK (job_type IN (
        'scan_antivirus',
        'transcode_hls',
        'convert_presentation_to_pdf'
    )),

    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN (
            'pending', 'processing', 'done', 'failed', 'dead_letter'
        )),

    attempts     INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INT NOT NULL DEFAULT 3 CHECK (max_attempts > 0),
    last_error   TEXT,

    -- Clave de idempotencia asynq: dos encolados con la misma clave no
    -- deben producir dos salidas. Ver condición de aceptación, sección 6.
    idempotency_key TEXT NOT NULL,

    locked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (idempotency_key)
);

CREATE INDEX idx_media_assets_resource_id ON media_assets (resource_id);
CREATE INDEX idx_upload_sessions_resource_id ON upload_sessions (resource_id);
CREATE INDEX idx_upload_sessions_status_expires ON upload_sessions (status, expires_at);
CREATE INDEX idx_media_jobs_media_asset_id ON media_jobs (media_asset_id);
CREATE INDEX idx_media_jobs_status ON media_jobs (status);
