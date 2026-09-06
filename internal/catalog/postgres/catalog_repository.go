package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/equipo-mooc/plataforma-mooc/internal/catalog/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/pagination"
)

type CatalogRepository struct {
	db *pgxpool.Pool
}

func NewCatalogRepository(db *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{db: db}
}

func wrapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// cursorPayload junta published_at + id para poder ordenar de forma
// estable (published_at solo no alcanza porque dos cursos pueden
// publicarse en el mismo instante).
func encodeCatalogCursor(publishedAt time.Time, id string) string {
	return pagination.EncodeCursor(fmt.Sprintf("%s|%s", publishedAt.Format(time.RFC3339Nano), id))
}

func decodeCatalogCursor(cursor string) (time.Time, string, error) {
	raw, err := pagination.DecodeCursor(cursor)
	if err != nil || raw == "" {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("cursor invalido")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	return t, parts[1], nil
}

func (r *CatalogRepository) SearchPublishedCourses(filter domain.SearchFilter) ([]domain.CourseSummary, string, error) {
	limit := pagination.NormalizeLimit(filter.Limit)

	cursorTime, cursorID, err := decodeCatalogCursor(filter.Cursor)
	if err != nil {
		return nil, "", fmt.Errorf("cursor invalido: %w", err)
	}

	query := `
		SELECT c.id, c.slug, cv.title, cv.summary, cv.category, cv.published_at
		FROM courses c
		JOIN course_versions cv ON cv.id = c.published_version_id
		WHERE c.published_version_id IS NOT NULL
	`
	args := []any{}
	argN := 1

	if filter.Query != "" {
		query += fmt.Sprintf(" AND (cv.title ILIKE $%d OR cv.summary ILIKE $%d)", argN, argN+1)
		args = append(args, "%"+filter.Query+"%", "%"+filter.Query+"%")
		argN += 2
	}
	if filter.Category != "" {
		query += fmt.Sprintf(" AND cv.category = $%d", argN)
		args = append(args, filter.Category)
		argN++
	}
	if !cursorTime.IsZero() {
		query += fmt.Sprintf(" AND (cv.published_at, c.id) < ($%d, $%d)", argN, argN+1)
		args = append(args, cursorTime, cursorID)
		argN += 2
	}
	query += fmt.Sprintf(" ORDER BY cv.published_at DESC, c.id DESC LIMIT $%d", argN)
	args = append(args, limit)

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []domain.CourseSummary
	for rows.Next() {
		var s domain.CourseSummary
		if err := rows.Scan(&s.CourseID, &s.Slug, &s.Title, &s.Summary, &s.Category, &s.PublishedAt); err != nil {
			return nil, "", err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(out) == limit {
		last := out[len(out)-1]
		nextCursor = encodeCatalogCursor(last.PublishedAt, last.CourseID)
	}
	return out, nextCursor, nil
}

func (r *CatalogRepository) FindPublishedCourseByID(courseID string) (*domain.CourseSummary, error) {
	var s domain.CourseSummary
	err := r.db.QueryRow(context.Background(), `
		SELECT c.id, c.slug, cv.title, cv.summary, cv.category, cv.published_at
		FROM courses c
		JOIN course_versions cv ON cv.id = c.published_version_id
		WHERE c.id = $1 AND c.published_version_id IS NOT NULL
	`, courseID).Scan(&s.CourseID, &s.Slug, &s.Title, &s.Summary, &s.Category, &s.PublishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCourseNotPublished
		}
		return nil, err
	}
	return &s, nil
}

// ---------- inscripciones ----------

func (r *CatalogRepository) FindEnrollment(studentID, courseID string) (*domain.Enrollment, error) {
	var e domain.Enrollment
	var withdrawnAt *time.Time
	err := r.db.QueryRow(context.Background(), `
		SELECT id, student_id, course_id, status, enrolled_at, withdrawn_at, created_at, updated_at
		FROM enrollments WHERE student_id = $1 AND course_id = $2
	`, studentID, courseID).Scan(&e.ID, &e.StudentID, &e.CourseID, &e.Status, &e.EnrolledAt, &withdrawnAt, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	e.WithdrawnAt = withdrawnAt
	return &e, nil
}

func (r *CatalogRepository) CreateEnrollment(e *domain.Enrollment) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO enrollments (student_id, course_id, status)
		VALUES ($1, $2, 'active')
		RETURNING id, enrolled_at, created_at, updated_at
	`, e.StudentID, e.CourseID).Scan(&e.ID, &e.EnrolledAt, &e.CreatedAt, &e.UpdatedAt)
}

func (r *CatalogRepository) ReactivateEnrollment(id string) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE enrollments SET status = 'active', enrolled_at = now(), withdrawn_at = NULL, updated_at = now()
		WHERE id = $1
	`, id)
	return err
}

func (r *CatalogRepository) WithdrawEnrollment(id string) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE enrollments SET status = 'withdrawn', withdrawn_at = now(), updated_at = now()
		WHERE id = $1
	`, id)
	return err
}

func (r *CatalogRepository) ListEnrollmentsByStudent(studentID string) ([]domain.Enrollment, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, student_id, course_id, status, enrolled_at, withdrawn_at, created_at, updated_at
		FROM enrollments WHERE student_id = $1 ORDER BY created_at DESC
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Enrollment
	for rows.Next() {
		var e domain.Enrollment
		var withdrawnAt *time.Time
		if err := rows.Scan(&e.ID, &e.StudentID, &e.CourseID, &e.Status, &e.EnrolledAt, &withdrawnAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.WithdrawnAt = withdrawnAt
		out = append(out, e)
	}
	return out, rows.Err()
}
