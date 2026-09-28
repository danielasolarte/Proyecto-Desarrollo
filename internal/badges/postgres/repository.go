package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/equipo-mooc/plataforma-mooc/internal/badges/domain"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) CreateBadge(
	ctx context.Context,
	badge *domain.Badge,
) error {

	query := `
		INSERT INTO badges (
			id,
			course_id,
			name,
			description,
			image_url,
			active,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			now(), now()
		)
		RETURNING created_at, updated_at
	`

	if badge.ID == uuid.Nil {
		badge.ID = uuid.New()
	}

	return r.db.QueryRow(
		ctx,
		query,
		badge.ID,
		badge.CourseID,
		badge.Name,
		badge.Description,
		badge.ImageURL,
		badge.Active,
	).Scan(
		&badge.CreatedAt,
		&badge.UpdatedAt,
	)
}

func (r *Repository) GetBadgeByCourseID(
	ctx context.Context,
	courseID uuid.UUID,
) (*domain.Badge, error) {

	query := `
		SELECT
			id,
			course_id,
			name,
			description,
			image_url,
			active,
			created_at,
			updated_at
		FROM badges
		WHERE course_id = $1
	`

	var badge domain.Badge

	err := r.db.QueryRow(
		ctx,
		query,
		courseID,
	).Scan(
		&badge.ID,
		&badge.CourseID,
		&badge.Name,
		&badge.Description,
		&badge.ImageURL,
		&badge.Active,
		&badge.CreatedAt,
		&badge.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrBadgeNotFound
	}

	if err != nil {
		return nil, err
	}

	return &badge, nil
}

func (r *Repository) CreateIssuance(
	ctx context.Context,
	issuance *domain.BadgeIssuance,
) error {

	query := `
		INSERT INTO badge_issuances (
			id,
			badge_id,
			student_id,
			course_id,
			enrollment_id,
			verification_code,
			issued_at,
			revoked_at,
			created_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, now(), $7, now()
		)
		RETURNING issued_at, created_at
	`

	if issuance.ID == uuid.Nil {
		issuance.ID = uuid.New()
	}

	if issuance.VerificationCode == uuid.Nil {
		issuance.VerificationCode = uuid.New()
	}

	return r.db.QueryRow(
		ctx,
		query,
		issuance.ID,
		issuance.BadgeID,
		issuance.StudentID,
		issuance.CourseID,
		issuance.EnrollmentID,
		issuance.VerificationCode,
		issuance.RevokedAt,
	).Scan(
		&issuance.IssuedAt,
		&issuance.CreatedAt,
	)
}

func (r *Repository) GetIssuanceByStudentAndCourse(
	ctx context.Context,
	studentID uuid.UUID,
	courseID uuid.UUID,
) (*domain.BadgeIssuance, error) {

	query := `
		SELECT
			id,
			badge_id,
			student_id,
			course_id,
			enrollment_id,
			verification_code,
			issued_at,
			revoked_at,
			created_at
		FROM badge_issuances
		WHERE student_id = $1
		  AND course_id = $2
	`

	var issuance domain.BadgeIssuance

	err := r.db.QueryRow(
		ctx,
		query,
		studentID,
		courseID,
	).Scan(
		&issuance.ID,
		&issuance.BadgeID,
		&issuance.StudentID,
		&issuance.CourseID,
		&issuance.EnrollmentID,
		&issuance.VerificationCode,
		&issuance.IssuedAt,
		&issuance.RevokedAt,
		&issuance.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrIssuanceNotFound
	}

	if err != nil {
		return nil, err
	}

	return &issuance, nil
}

func (r *Repository) GetIssuanceByVerificationCode(
	ctx context.Context,
	code uuid.UUID,
) (*domain.BadgeIssuance, error) {

	query := `
		SELECT
			id,
			badge_id,
			student_id,
			course_id,
			enrollment_id,
			verification_code,
			issued_at,
			revoked_at,
			created_at
		FROM badge_issuances
		WHERE verification_code = $1
	`

	var issuance domain.BadgeIssuance

	err := r.db.QueryRow(
		ctx,
		query,
		code,
	).Scan(
		&issuance.ID,
		&issuance.BadgeID,
		&issuance.StudentID,
		&issuance.CourseID,
		&issuance.EnrollmentID,
		&issuance.VerificationCode,
		&issuance.IssuedAt,
		&issuance.RevokedAt,
		&issuance.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrIssuanceNotFound
	}

	if err != nil {
		return nil, err
	}

	return &issuance, nil
}

func (r *Repository) RevokeIssuance(
	ctx context.Context,
	issuanceID uuid.UUID,
) error {

	result, err := r.db.Exec(
		ctx,
		`
		UPDATE badge_issuances
		SET revoked_at = now()
		WHERE id = $1
		  AND revoked_at IS NULL
		`,
		issuanceID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrIssuanceNotFound
	}

	return nil
}

func (r *Repository) GetBadgeByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.Badge, error) {

	query := `
		SELECT
			id,
			course_id,
			name,
			description,
			image_url,
			active,
			created_at,
			updated_at
		FROM badges
		WHERE id = $1
	`

	var badge domain.Badge

	err := r.db.QueryRow(
		ctx,
		query,
		id,
	).Scan(
		&badge.ID,
		&badge.CourseID,
		&badge.Name,
		&badge.Description,
		&badge.ImageURL,
		&badge.Active,
		&badge.CreatedAt,
		&badge.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrBadgeNotFound
	}

	if err != nil {
		return nil, err
	}

	return &badge, nil
}

