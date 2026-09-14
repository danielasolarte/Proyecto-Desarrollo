package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

// ValidatePublication revisa las condiciones de la sección 6 del
// enunciado ("Publicación válida"): metadatos completos, estructura
// mínima, y todos los recursos visibles disponibles. Devuelve la lista
// COMPLETA de errores encontrados (no solo el primero), porque el
// enunciado pide una "lista exhaustiva de errores" en la previsualización.
func ValidatePublication(tree *domain.CourseTree) []string {
	problems := []string{}

	v := tree.Version
	if strings.TrimSpace(v.Title) == "" {
		problems = append(problems, "el curso no tiene titulo")
	}
	if strings.TrimSpace(v.Summary) == "" {
		problems = append(problems, "el curso no tiene resumen")
	}
	if strings.TrimSpace(v.Category) == "" {
		problems = append(problems, "el curso no tiene categoria")
	}

	if len(tree.Modules) == 0 {
		problems = append(problems, "el curso no tiene ningun modulo")
	}

	visibleResourceCount := 0
	for _, modTree := range tree.Modules {
		if strings.TrimSpace(modTree.Module.Title) == "" {
			problems = append(problems, fmt.Sprintf("el modulo en la posicion %d no tiene titulo", modTree.Module.Position))
		}
		if len(modTree.Units) == 0 {
			problems = append(problems, fmt.Sprintf("el modulo %q no tiene ninguna unidad", modTree.Module.Title))
		}
		for _, unitTree := range modTree.Units {
			if strings.TrimSpace(unitTree.Unit.Title) == "" {
				problems = append(problems, fmt.Sprintf("la unidad en la posicion %d no tiene titulo", unitTree.Unit.Position))
			}
			hasVisible := false
			for _, res := range unitTree.Resources {
				if !res.Visible {
					continue
				}
				hasVisible = true
				visibleResourceCount++
				if res.Type.IsMedia() {
					if res.ProcessingStatus == nil || *res.ProcessingStatus != domain.ProcessingReady {
						problems = append(problems, fmt.Sprintf("el recurso %q todavia no esta listo (procesamiento pendiente)", res.Title))
					}
				}
				if res.Type == domain.ResourceRichText && (res.ContentMarkdown == nil || strings.TrimSpace(*res.ContentMarkdown) == "") {
					problems = append(problems, fmt.Sprintf("el recurso %q no tiene contenido publicado", res.Title))
				}
			}
			if !hasVisible {
				problems = append(problems, fmt.Sprintf("la unidad %q no tiene ningun recurso visible", unitTree.Unit.Title))
			}
		}
	}
	if visibleResourceCount == 0 {
		problems = append(problems, "el curso no tiene ningun recurso visible en toda su estructura")
	}

	return problems
}

// PublishVersion valida y, si todo está en orden, publica la versión
// draft de un curso. Si el curso ya tenía una versión publicada, esa
// versión pasa a "superseded" y el progreso de los estudiantes sigue
// aplicando via stable_id (lo administra el módulo de progreso).
func PublishVersion(repo domain.CourseRepository, courseID string) (*domain.CourseVersion, error) {
	course, err := repo.FindCourseByID(courseID)
	if err != nil {
		return nil, err
	}
	if course.LatestDraftVersionID == nil {
		return nil, errors.Join(domain.ErrValidation, errors.New("el curso no tiene un borrador para publicar"))
	}

	tree, err := repo.LoadTree(*course.LatestDraftVersionID)
	if err != nil {
		return nil, err
	}
	if problems := ValidatePublication(tree); len(problems) > 0 {
		return nil, errors.Join(domain.ErrValidation, errors.New(strings.Join(problems, "; ")))
	}

	now := time.Now()
	if err := repo.MarkVersionPublished(tree.Version.ID, now); err != nil {
		return nil, err
	}

	if course.PublishedVersionID != nil {
		if err := repo.MarkVersionSuperseded(*course.PublishedVersionID); err != nil {
			return nil, err
		}
	}

	newPublished := *course.LatestDraftVersionID
	if err := repo.UpdateCoursePublishedVersion(courseID, &newPublished, nil); err != nil {
		return nil, err
	}

	tree.Version.Status = domain.VersionStatusPublished
	tree.Version.PublishedAt = &now
	return tree.Version, nil
}

// Preview devuelve el árbol completo de la versión que se debe mostrar en
// la previsualización: el draft si existe, o la publicada si no hay
// draft en curso.
func Preview(repo domain.CourseRepository, courseID string) (*domain.CourseTree, []string, error) {
	course, err := repo.FindCourseByID(courseID)
	if err != nil {
		return nil, nil, err
	}

	var versionID string
	switch {
	case course.LatestDraftVersionID != nil:
		versionID = *course.LatestDraftVersionID
	case course.PublishedVersionID != nil:
		versionID = *course.PublishedVersionID
	default:
		return nil, nil, errors.New("el curso no tiene ninguna version")
	}

	tree, err := repo.LoadTree(versionID)
	if err != nil {
		return nil, nil, err
	}

	problems := []string{}
	if tree.Version.Status == domain.VersionStatusDraft {
		problems = ValidatePublication(tree)
	}
	return tree, problems, nil
}
