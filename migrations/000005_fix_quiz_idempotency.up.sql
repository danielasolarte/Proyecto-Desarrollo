-- Elimina posibles duplicados existentes antes de crear la restricción.
WITH ranked_events AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY
                student_id,
                enrollment_id,
                resource_stable_id,
                event_type
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM progress_events
    WHERE event_type IN ('quiz_submitted', 'quiz_passed')
)
DELETE FROM progress_events
WHERE id IN (
    SELECT id
    FROM ranked_events
    WHERE rn > 1
);

-- Solo puede existir un evento académico de cada tipo
-- por estudiante, inscripción y recurso estable.
CREATE UNIQUE INDEX idx_progress_events_unique_quiz_event
ON progress_events (
    student_id,
    enrollment_id,
    resource_stable_id,
    event_type
)
WHERE event_type IN ('quiz_submitted', 'quiz_passed');