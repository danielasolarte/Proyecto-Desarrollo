package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/equipo-mooc/plataforma-mooc/internal/progress/domain"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) GetResourceProgress(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceStableID uuid.UUID,
) (*domain.ResourceProgress, error) {

	query := `
		SELECT
			id,
			student_id,
			enrollment_id,
			resource_id,
			resource_stable_id,
			status,
			opened_at,
			last_activity_at,
			completed_at,
			active_seconds,
			last_position_seconds,
			created_at,
			updated_at
		FROM resource_progress
		WHERE student_id = $1
		  AND enrollment_id = $2
		  AND resource_stable_id = $3
	`

	var progress domain.ResourceProgress

	err := r.db.QueryRow(
		ctx,
		query,
		studentID,
		enrollmentID,
		resourceStableID,
	).Scan(
		&progress.ID,
		&progress.StudentID,
		&progress.EnrollmentID,
		&progress.ResourceID,
		&progress.ResourceStableID,
		&progress.Status,
		&progress.OpenedAt,
		&progress.LastActivityAt,
		&progress.CompletedAt,
		&progress.ActiveSeconds,
		&progress.LastPositionSeconds,
		&progress.CreatedAt,
		&progress.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProgressNotFound
	}

	if err != nil {
		return nil, err
	}

	return &progress, nil
}

func (r *Repository) UpsertResourceProgress(
	ctx context.Context,
	progress *domain.ResourceProgress,
) error {

	query := `
		INSERT INTO resource_progress (
			id,
			student_id,
			enrollment_id,
			resource_id,
			resource_stable_id,
			status,
			opened_at,
			last_activity_at,
			completed_at,
			active_seconds,
			last_position_seconds,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, now(), now()
		)
		ON CONFLICT (
			student_id,
			enrollment_id,
			resource_stable_id
		)
		DO UPDATE SET
			resource_id = EXCLUDED.resource_id,
			status = EXCLUDED.status,
			opened_at = COALESCE(
				resource_progress.opened_at,
				EXCLUDED.opened_at
			),
			last_activity_at = EXCLUDED.last_activity_at,
			completed_at = EXCLUDED.completed_at,
			active_seconds = EXCLUDED.active_seconds,
			last_position_seconds = EXCLUDED.last_position_seconds,
			updated_at = now()
		RETURNING
			id,
			created_at,
			updated_at
	`

	if progress.ID == uuid.Nil {
		progress.ID = uuid.New()
	}

	return r.db.QueryRow(
		ctx,
		query,
		progress.ID,
		progress.StudentID,
		progress.EnrollmentID,
		progress.ResourceID,
		progress.ResourceStableID,
		progress.Status,
		progress.OpenedAt,
		progress.LastActivityAt,
		progress.CompletedAt,
		progress.ActiveSeconds,
		progress.LastPositionSeconds,
	).Scan(
		&progress.ID,
		&progress.CreatedAt,
		&progress.UpdatedAt,
	)
}

func (r *Repository) CreateProgressEvent(
	ctx context.Context,
	event *domain.ProgressEvent,
) error {

	query := `
		INSERT INTO progress_events (
			id,
			student_id,
			enrollment_id,
			resource_id,
			resource_stable_id,
			event_type,
			position_seconds,
			client_timestamp,
			created_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, now()
		)
		ON CONFLICT (
			student_id,
			enrollment_id,
			resource_stable_id,
			event_type
		)
		WHERE event_type IN ('quiz_submitted', 'quiz_passed')
		DO NOTHING
		RETURNING created_at
	`

	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	err := r.db.QueryRow(
		ctx,
		query,
		event.ID,
		event.StudentID,
		event.EnrollmentID,
		event.ResourceID,
		event.ResourceStableID,
		event.EventType,
		event.PositionSeconds,
		event.ClientTimestamp,
	).Scan(&event.CreatedAt)

	// Si otra request creó el mismo quiz_submitted o quiz_passed
	// simultáneamente, la operación sigue siendo exitosa.
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}

	return err
}


func (r *Repository) GetCourseProgress(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	courseID uuid.UUID,
) (*domain.CourseProgress, error) {

	query := `
		SELECT
			id,
			student_id,
			enrollment_id,
			course_id,
			status,
			progress_percentage,
			completed_at,
			approved_at,
			created_at,
			updated_at
		FROM course_progress
		WHERE student_id = $1
		  AND enrollment_id = $2
		  AND course_id = $3
	`

	var progress domain.CourseProgress

	err := r.db.QueryRow(
		ctx,
		query,
		studentID,
		enrollmentID,
		courseID,
	).Scan(
		&progress.ID,
		&progress.StudentID,
		&progress.EnrollmentID,
		&progress.CourseID,
		&progress.Status,
		&progress.ProgressPercentage,
		&progress.CompletedAt,
		&progress.ApprovedAt,
		&progress.CreatedAt,
		&progress.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProgressNotFound
	}

	if err != nil {
		return nil, err
	}

	return &progress, nil
}

func (r *Repository) UpsertCourseProgress(
	ctx context.Context,
	progress *domain.CourseProgress,
) error {

	query := `
		INSERT INTO course_progress (
			id,
			student_id,
			enrollment_id,
			course_id,
			status,
			progress_percentage,
			completed_at,
			approved_at,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, now(), now()
		)
		ON CONFLICT (
			student_id,
			enrollment_id,
			course_id
		)
		DO UPDATE SET
			status = EXCLUDED.status,
			progress_percentage = EXCLUDED.progress_percentage,
			completed_at = EXCLUDED.completed_at,
			approved_at = EXCLUDED.approved_at,
			updated_at = now()
		RETURNING
			id,
			created_at,
			updated_at
	`

	if progress.ID == uuid.Nil {
		progress.ID = uuid.New()
	}

	return r.db.QueryRow(
		ctx,
		query,
		progress.ID,
		progress.StudentID,
		progress.EnrollmentID,
		progress.CourseID,
		progress.Status,
		progress.ProgressPercentage,
		progress.CompletedAt,
		progress.ApprovedAt,
	).Scan(
		&progress.ID,
		&progress.CreatedAt,
		&progress.UpdatedAt,
	)
}

func (r *Repository) ListCompletedResourceStableIDs(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
) ([]uuid.UUID, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT resource_stable_id
		FROM resource_progress
		WHERE student_id = $1
		  AND enrollment_id = $2
		  AND status = 'completed'
		`,
		studentID,
		enrollmentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stableIDs []uuid.UUID

	for rows.Next() {
		var stableID uuid.UUID

		if err := rows.Scan(&stableID); err != nil {
			return nil, err
		}

		stableIDs = append(stableIDs, stableID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stableIDs, nil
}

func (r *Repository) ListPassedQuizStableIDs(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
) ([]uuid.UUID, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT DISTINCT resource_stable_id
		FROM progress_events
		WHERE student_id = $1
		  AND enrollment_id = $2
		  AND event_type = 'quiz_passed'
		`,
		studentID,
		enrollmentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stableIDs []uuid.UUID

	for rows.Next() {
		var stableID uuid.UUID

		if err := rows.Scan(&stableID); err != nil {
			return nil, err
		}

		stableIDs = append(stableIDs, stableID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stableIDs, nil
}

func (r *Repository) HasProgressEvent(
	ctx context.Context,
	studentID uuid.UUID,
	enrollmentID uuid.UUID,
	resourceStableID uuid.UUID,
	eventType string,
) (bool, error) {

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM progress_events
			WHERE student_id = $1
			  AND enrollment_id = $2
			  AND resource_stable_id = $3
			  AND event_type = $4
		)
	`

	var exists bool

	err := r.db.QueryRow(
		ctx,
		query,
		studentID,
		enrollmentID,
		resourceStableID,
		eventType,
	).Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists, nil
}