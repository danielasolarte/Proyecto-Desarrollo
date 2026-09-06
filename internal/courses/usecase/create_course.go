package usecase

import (
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

type CreateCourseInput struct {
	TeacherID string
	Title     string
	Summary   string
	Category  string
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "curso"
	}
	return s
}

// CreateCourse crea el curso y, junto con él, su primera versión en
// estado draft (version_number = 1). Un curso nunca existe sin al menos
// una versión.
func CreateCourse(repo domain.CourseRepository, in CreateCourseInput) (*domain.Course, *domain.CourseVersion, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, nil, errors.Join(domain.ErrValidation, errors.New("el titulo es obligatorio"))
	}
	if strings.TrimSpace(in.TeacherID) == "" {
		return nil, nil, errors.Join(domain.ErrValidation, errors.New("falta el profesor propietario"))
	}

	slug := slugify(in.Title)
	if _, err := repo.FindCourseBySlug(slug); err == nil {
		// ya existe un curso con ese slug: se le agrega un sufijo corto
		slug = slug + "-" + uuid.NewString()[:8]
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, nil, err
	}

	course := &domain.Course{
		TeacherID: in.TeacherID,
		Slug:      slug,
	}
	version := &domain.CourseVersion{
		Title:    in.Title,
		Summary:  in.Summary,
		Category: in.Category,
	}

	if err := repo.CreateCourseWithFirstVersion(course, version); err != nil {
		return nil, nil, err
	}
	return course, version, nil
}
