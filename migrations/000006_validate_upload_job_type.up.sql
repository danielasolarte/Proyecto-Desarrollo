-- Entrega 5: la validación de la carga (checksum real, MIME real y firma de
-- las presentaciones) pasa a ser su propio tipo de trabajo, validate_upload,
-- que ejecuta la Cloud Function mooc-validar-carga.
--
-- La restricción se creó en 000004 sin nombre explícito, así que Postgres le
-- puso el nombre por defecto media_jobs_job_type_check.
ALTER TABLE media_jobs DROP CONSTRAINT IF EXISTS media_jobs_job_type_check;

ALTER TABLE media_jobs ADD CONSTRAINT media_jobs_job_type_check
    CHECK (job_type IN (
        'validate_upload',
        'scan_antivirus',
        'transcode_hls',
        'convert_presentation_to_pdf'
    ));
