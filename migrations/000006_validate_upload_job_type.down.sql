-- Revierte 000006. Los trabajos validate_upload no caben en la restricción
-- anterior, así que se eliminan antes de volver a crearla. Sus activos
-- quedan como estaban (el trabajo de antivirus, si ya se encoló, sigue).
DELETE FROM media_jobs WHERE job_type = 'validate_upload';

ALTER TABLE media_jobs DROP CONSTRAINT IF EXISTS media_jobs_job_type_check;

ALTER TABLE media_jobs ADD CONSTRAINT media_jobs_job_type_check
    CHECK (job_type IN (
        'scan_antivirus',
        'transcode_hls',
        'convert_presentation_to_pdf'
    ));
