package usecase

import (
	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

// PublishedOutline arma el temario público de la versión PUBLICADA de un
// curso. Si el curso no está publicado responde ErrNotFound (no se revela
// que existe un borrador). Los recursos ocultos no aparecen y nunca se
// incluye su contenido.
func PublishedOutline(repo domain.CourseRepository, courseID string) (*domain.CourseOutline, error) {
	course, err := repo.FindCourseByID(courseID)
	if err != nil {
		return nil, err
	}
	if course.PublishedVersionID == nil {
		return nil, domain.ErrNotFound
	}
	tree, err := repo.LoadTree(*course.PublishedVersionID)
	if err != nil {
		return nil, err
	}

	out := &domain.CourseOutline{
		CourseID:    course.ID,
		Slug:        course.Slug,
		TeacherID:   course.TeacherID,
		Title:       tree.Version.Title,
		Summary:     tree.Version.Summary,
		Category:    tree.Version.Category,
		PublishedAt: tree.Version.PublishedAt,
		Modules:     []domain.OutlineModule{},
	}
	for _, m := range tree.Modules {
		om := domain.OutlineModule{ID: m.Module.ID, Title: m.Module.Title, Units: []domain.OutlineUnit{}}
		for _, u := range m.Units {
			ou := domain.OutlineUnit{ID: u.Unit.ID, Title: u.Unit.Title, Resources: []domain.OutlineResource{}}
			for _, r := range u.Resources {
				if !r.Visible {
					continue
				}
				ou.Resources = append(ou.Resources, domain.OutlineResource{ID: r.ID, Title: r.Title, Type: r.Type, Required: r.Required})
			}
			om.Units = append(om.Units, ou)
		}
		out.Modules = append(out.Modules, om)
	}
	return out, nil
}
