// Package postgres implementa domain.CourseRepository contra PostgreSQL
// usando pgx. Es la única capa del módulo que sabe escribir SQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

type CourseRepository struct {
	db *pgxpool.Pool
}

func NewCourseRepository(db *pgxpool.Pool) *CourseRepository {
	return &CourseRepository{db: db}
}

// ---------- helpers de nulabilidad ----------
// Se usan sql.NullString/sql.NullTime en vez de *string/*time.Time
// directamente contra pgx porque implementan sql.Scanner y driver.Valuer,
// lo que hace el comportamiento con NULL predecible tanto al leer como al
// escribir.

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

func changeTypePtr(ns sql.NullString) *domain.ChangeType {
	if !ns.Valid {
		return nil
	}
	v := domain.ChangeType(ns.String)
	return &v
}

func nullableChangeType(c *domain.ChangeType) sql.NullString {
	if c == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*c), Valid: true}
}

func processingStatusPtr(ns sql.NullString) *domain.ProcessingStatus {
	if !ns.Valid {
		return nil
	}
	v := domain.ProcessingStatus(ns.String)
	return &v
}

func nullableProcessingStatus(p *domain.ProcessingStatus) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*p), Valid: true}
}

func wrapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// ---------- courses ----------

func (r *CourseRepository) CreateCourseWithFirstVersion(course *domain.Course, version *domain.CourseVersion) error {
	ctx := context.Background()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO courses (teacher_id, slug)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`, course.TeacherID, course.Slug).Scan(&course.ID, &course.CreatedAt, &course.UpdatedAt)
	if err != nil {
		return err
	}

	version.CourseID = course.ID
	version.VersionNumber = 1
	version.Status = domain.VersionStatusDraft
	err = tx.QueryRow(ctx, `
		INSERT INTO course_versions (course_id, version_number, status, title, summary, category)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, version.CourseID, version.VersionNumber, version.Status, version.Title, version.Summary, version.Category).
		Scan(&version.ID, &version.CreatedAt, &version.UpdatedAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `UPDATE courses SET latest_draft_version_id = $1 WHERE id = $2`, version.ID, course.ID)
	if err != nil {
		return err
	}
	course.LatestDraftVersionID = &version.ID

	return tx.Commit(ctx)
}

func (r *CourseRepository) FindCourseByID(id string) (*domain.Course, error) {
	var c domain.Course
	var published, draft sql.NullString
	err := r.db.QueryRow(context.Background(), `
		SELECT id, teacher_id, slug, published_version_id, latest_draft_version_id, created_at, updated_at
		FROM courses WHERE id = $1
	`, id).Scan(&c.ID, &c.TeacherID, &c.Slug, &published, &draft, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	c.PublishedVersionID = strPtr(published)
	c.LatestDraftVersionID = strPtr(draft)
	return &c, nil
}

func (r *CourseRepository) FindCourseBySlug(slug string) (*domain.Course, error) {
	var c domain.Course
	var published, draft sql.NullString
	err := r.db.QueryRow(context.Background(), `
		SELECT id, teacher_id, slug, published_version_id, latest_draft_version_id, created_at, updated_at
		FROM courses WHERE slug = $1
	`, slug).Scan(&c.ID, &c.TeacherID, &c.Slug, &published, &draft, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	c.PublishedVersionID = strPtr(published)
	c.LatestDraftVersionID = strPtr(draft)
	return &c, nil
}

func (r *CourseRepository) UpdateCoursePublishedVersion(courseID string, publishedVersionID *string, latestDraftVersionID *string) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE courses SET published_version_id = $1, latest_draft_version_id = $2, updated_at = now()
		WHERE id = $3
	`, nullableStr(publishedVersionID), nullableStr(latestDraftVersionID), courseID)
	return err
}

// ---------- course_versions ----------

func (r *CourseRepository) FindVersionByID(id string) (*domain.CourseVersion, error) {
	var v domain.CourseVersion
	var changeType, category, summary sql.NullString
	var publishedAt sql.NullTime
	err := r.db.QueryRow(context.Background(), `
		SELECT id, course_id, version_number, status, title, summary, category, change_type, published_at, created_at, updated_at
		FROM course_versions WHERE id = $1
	`, id).Scan(&v.ID, &v.CourseID, &v.VersionNumber, &v.Status, &v.Title, &summary, &category, &changeType, &publishedAt, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	v.Summary = summary.String
	v.Category = category.String
	v.ChangeType = changeTypePtr(changeType)
	v.PublishedAt = timePtr(publishedAt)
	return &v, nil
}

func (r *CourseRepository) FindPublishedVersion(courseID string) (*domain.CourseVersion, error) {
	var v domain.CourseVersion
	var changeType, category, summary sql.NullString
	var publishedAt sql.NullTime
	err := r.db.QueryRow(context.Background(), `
		SELECT cv.id, cv.course_id, cv.version_number, cv.status, cv.title, cv.summary, cv.category, cv.change_type, cv.published_at, cv.created_at, cv.updated_at
		FROM course_versions cv
		JOIN courses c ON c.published_version_id = cv.id
		WHERE c.id = $1
	`, courseID).Scan(&v.ID, &v.CourseID, &v.VersionNumber, &v.Status, &v.Title, &summary, &category, &changeType, &publishedAt, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	v.Summary = summary.String
	v.Category = category.String
	v.ChangeType = changeTypePtr(changeType)
	v.PublishedAt = timePtr(publishedAt)
	return &v, nil
}

func (r *CourseRepository) FindDraftVersion(courseID string) (*domain.CourseVersion, error) {
	var v domain.CourseVersion
	var changeType, category, summary sql.NullString
	var publishedAt sql.NullTime
	err := r.db.QueryRow(context.Background(), `
		SELECT cv.id, cv.course_id, cv.version_number, cv.status, cv.title, cv.summary, cv.category, cv.change_type, cv.published_at, cv.created_at, cv.updated_at
		FROM course_versions cv
		JOIN courses c ON c.latest_draft_version_id = cv.id
		WHERE c.id = $1 AND cv.status = 'draft'
	`, courseID).Scan(&v.ID, &v.CourseID, &v.VersionNumber, &v.Status, &v.Title, &summary, &category, &changeType, &publishedAt, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	v.Summary = summary.String
	v.Category = category.String
	v.ChangeType = changeTypePtr(changeType)
	v.PublishedAt = timePtr(publishedAt)
	return &v, nil
}

func (r *CourseRepository) UpdateVersionMetadata(v *domain.CourseVersion) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE course_versions
		SET title = $1, summary = $2, category = $3, updated_at = now()
		WHERE id = $4 AND status = 'draft'
	`, v.Title, v.Summary, v.Category, v.ID)
	return err
}

func (r *CourseRepository) MarkVersionPublished(versionID string, publishedAt time.Time) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE course_versions SET status = 'published', published_at = $1, updated_at = now()
		WHERE id = $2
	`, publishedAt, versionID)
	return err
}

func (r *CourseRepository) MarkVersionSuperseded(versionID string) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE course_versions SET status = 'superseded', updated_at = now() WHERE id = $1
	`, versionID)
	return err
}

func (r *CourseRepository) CreateVersion(v *domain.CourseVersion) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO course_versions (course_id, version_number, status, title, summary, category, change_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`, v.CourseID, v.VersionNumber, v.Status, v.Title, v.Summary, v.Category, nullableChangeType(v.ChangeType)).
		Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
}

func (r *CourseRepository) NextVersionNumber(courseID string) (int, error) {
	var max int
	err := r.db.QueryRow(context.Background(), `
		SELECT COALESCE(MAX(version_number), 0) FROM course_versions WHERE course_id = $1
	`, courseID).Scan(&max)
	if err != nil {
		return 0, err
	}
	return max + 1, nil
}

// ---------- modules ----------

func (r *CourseRepository) CreateModule(m *domain.Module) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO modules (course_version_id, stable_id, title, position)
		VALUES ($1, COALESCE($2, gen_random_uuid()), $3, $4)
		RETURNING id, stable_id, created_at, updated_at
	`, m.CourseVersionID, nullableStr(strOrNil(m.StableID)), m.Title, m.Position).
		Scan(&m.ID, &m.StableID, &m.CreatedAt, &m.UpdatedAt)
}

func (r *CourseRepository) FindModuleByID(id string) (*domain.Module, error) {
	var m domain.Module
	err := r.db.QueryRow(context.Background(), `
		SELECT id, course_version_id, stable_id, title, position, created_at, updated_at
		FROM modules WHERE id = $1
	`, id).Scan(&m.ID, &m.CourseVersionID, &m.StableID, &m.Title, &m.Position, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	return &m, nil
}

func (r *CourseRepository) ListModulesByVersion(versionID string) ([]domain.Module, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, course_version_id, stable_id, title, position, created_at, updated_at
		FROM modules WHERE course_version_id = $1 ORDER BY position ASC
	`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Module{}
	for rows.Next() {
		var m domain.Module
		if err := rows.Scan(&m.ID, &m.CourseVersionID, &m.StableID, &m.Title, &m.Position, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *CourseRepository) UpdateModule(m *domain.Module) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE modules SET title = $1, position = $2, updated_at = now() WHERE id = $3
	`, m.Title, m.Position, m.ID)
	return err
}

func (r *CourseRepository) DeleteModule(id string) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM modules WHERE id = $1`, id)
	return err
}

// ---------- units ----------

func (r *CourseRepository) CreateUnit(u *domain.Unit) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO units (module_id, stable_id, title, position)
		VALUES ($1, COALESCE($2, gen_random_uuid()), $3, $4)
		RETURNING id, stable_id, created_at, updated_at
	`, u.ModuleID, nullableStr(strOrNil(u.StableID)), u.Title, u.Position).
		Scan(&u.ID, &u.StableID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *CourseRepository) FindUnitByID(id string) (*domain.Unit, error) {
	var u domain.Unit
	err := r.db.QueryRow(context.Background(), `
		SELECT id, module_id, stable_id, title, position, created_at, updated_at
		FROM units WHERE id = $1
	`, id).Scan(&u.ID, &u.ModuleID, &u.StableID, &u.Title, &u.Position, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	return &u, nil
}

func (r *CourseRepository) ListUnitsByModule(moduleID string) ([]domain.Unit, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, module_id, stable_id, title, position, created_at, updated_at
		FROM units WHERE module_id = $1 ORDER BY position ASC
	`, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Unit{}
	for rows.Next() {
		var u domain.Unit
		if err := rows.Scan(&u.ID, &u.ModuleID, &u.StableID, &u.Title, &u.Position, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *CourseRepository) UpdateUnit(u *domain.Unit) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE units SET title = $1, position = $2, updated_at = now() WHERE id = $3
	`, u.Title, u.Position, u.ID)
	return err
}

func (r *CourseRepository) DeleteUnit(id string) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM units WHERE id = $1`, id)
	return err
}

// ---------- resources ----------

func (r *CourseRepository) CreateResource(res *domain.Resource) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO resources (
			unit_id, stable_id, type, title, position, visible, required, downloadable,
			processing_status, content_markdown, content_draft_markdown, draft_saved_at,
			external_url, media_asset_id
		)
		VALUES ($1, COALESCE($2, gen_random_uuid()), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, stable_id, created_at, updated_at
	`,
		res.UnitID, nullableStr(strOrNil(res.StableID)), res.Type, res.Title, res.Position,
		res.Visible, res.Required, res.Downloadable,
		nullableProcessingStatus(res.ProcessingStatus), nullableStr(res.ContentMarkdown),
		nullableStr(res.ContentDraftMarkdown), nullableTime(res.DraftSavedAt),
		nullableStr(res.ExternalURL), nullableStr(res.MediaAssetID),
	).Scan(&res.ID, &res.StableID, &res.CreatedAt, &res.UpdatedAt)
}

func (r *CourseRepository) FindResourceByID(id string) (*domain.Resource, error) {
	res, err := scanResourceRow(r.db.QueryRow(context.Background(), `
		SELECT id, unit_id, stable_id, type, title, position, visible, required, downloadable,
		       processing_status, content_markdown, content_draft_markdown, draft_saved_at,
		       external_url, media_asset_id, created_at, updated_at
		FROM resources WHERE id = $1
	`, id))
	if err != nil {
		return nil, wrapErr(err)
	}
	return res, nil
}

func (r *CourseRepository) ListResourcesByUnit(unitID string) ([]domain.Resource, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, unit_id, stable_id, type, title, position, visible, required, downloadable,
		       processing_status, content_markdown, content_draft_markdown, draft_saved_at,
		       external_url, media_asset_id, created_at, updated_at
		FROM resources WHERE unit_id = $1 ORDER BY position ASC
	`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Resource{}
	for rows.Next() {
		res, err := scanResourceRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *res)
	}
	return out, rows.Err()
}

func (r *CourseRepository) UpdateResource(res *domain.Resource) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE resources SET
			title = $1, position = $2, visible = $3, required = $4, downloadable = $5,
			processing_status = $6, content_markdown = $7, content_draft_markdown = $8,
			draft_saved_at = $9, external_url = $10, media_asset_id = $11, updated_at = now()
		WHERE id = $12
	`,
		res.Title, res.Position, res.Visible, res.Required, res.Downloadable,
		nullableProcessingStatus(res.ProcessingStatus), nullableStr(res.ContentMarkdown),
		nullableStr(res.ContentDraftMarkdown), nullableTime(res.DraftSavedAt),
		nullableStr(res.ExternalURL), nullableStr(res.MediaAssetID), res.ID,
	)
	return err
}

func (r *CourseRepository) DeleteResource(id string) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM resources WHERE id = $1`, id)
	return err
}

// rowScanner abstrae pgx.Row y pgx.Rows, que comparten el método Scan.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanResourceRow(row rowScanner) (*domain.Resource, error) {
	var res domain.Resource
	var processingStatus, contentMarkdown, contentDraft, externalURL, mediaAssetID sql.NullString
	var draftSavedAt sql.NullTime
	err := row.Scan(
		&res.ID, &res.UnitID, &res.StableID, &res.Type, &res.Title, &res.Position,
		&res.Visible, &res.Required, &res.Downloadable,
		&processingStatus, &contentMarkdown, &contentDraft, &draftSavedAt,
		&externalURL, &mediaAssetID, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	res.ProcessingStatus = processingStatusPtr(processingStatus)
	res.ContentMarkdown = strPtr(contentMarkdown)
	res.ContentDraftMarkdown = strPtr(contentDraft)
	res.DraftSavedAt = timePtr(draftSavedAt)
	res.ExternalURL = strPtr(externalURL)
	res.MediaAssetID = strPtr(mediaAssetID)
	return &res, nil
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------- árbol completo ----------

func (r *CourseRepository) LoadTree(versionID string) (*domain.CourseTree, error) {
	version, err := r.FindVersionByID(versionID)
	if err != nil {
		return nil, err
	}

	modules, err := r.ListModulesByVersion(versionID)
	if err != nil {
		return nil, err
	}

	tree := &domain.CourseTree{Version: version, Modules: []domain.ModuleTree{}}
	for i := range modules {
		mod := modules[i]
		units, err := r.ListUnitsByModule(mod.ID)
		if err != nil {
			return nil, err
		}
		modTree := domain.ModuleTree{Module: &mod, Units: []domain.UnitTree{}}
		for j := range units {
			unit := units[j]
			resources, err := r.ListResourcesByUnit(unit.ID)
			if err != nil {
				return nil, err
			}
			modTree.Units = append(modTree.Units, domain.UnitTree{Unit: &unit, Resources: resources})
		}
		tree.Modules = append(tree.Modules, modTree)
	}
	return tree, nil
}
