// Este archivo implementa el opcional "Borradores de actualización con
// clasificación de cambios" (sección 5.2 del enunciado): permite editar un
// curso YA PUBLICADO sin despublicarlo, creando una nueva versión en
// estado draft que es una copia exacta de la publicada. Los módulos,
// unidades y recursos copiados conservan su stable_id original, que es
// justo lo que el módulo de progreso necesita para no perder el avance
// de los estudiantes cuando se publique la actualización.
package usecase

import (
	"errors"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

type CreateUpdateDraftInput struct {
	CourseID   string
	ChangeType domain.ChangeType // "minor" o "major"
}

func CreateUpdateDraft(repo domain.CourseRepository, in CreateUpdateDraftInput) (*domain.CourseVersion, error) {
	if in.ChangeType != domain.ChangeTypeMinor && in.ChangeType != domain.ChangeTypeMajor {
		return nil, errors.Join(domain.ErrValidation, errors.New("change_type debe ser 'minor' o 'major'"))
	}

	course, err := repo.FindCourseByID(in.CourseID)
	if err != nil {
		return nil, err
	}
	if course.PublishedVersionID == nil {
		return nil, domain.ErrCourseHasNoPublished
	}
	if course.LatestDraftVersionID != nil {
		return nil, domain.ErrCourseAlreadyHasDraft
	}

	publishedTree, err := repo.LoadTree(*course.PublishedVersionID)
	if err != nil {
		return nil, err
	}

	nextNumber, err := repo.NextVersionNumber(in.CourseID)
	if err != nil {
		return nil, err
	}

	newVersion := &domain.CourseVersion{
		CourseID:      in.CourseID,
		VersionNumber: nextNumber,
		Status:        domain.VersionStatusDraft,
		Title:         publishedTree.Version.Title,
		Summary:       publishedTree.Version.Summary,
		Category:      publishedTree.Version.Category,
		ChangeType:    &in.ChangeType,
	}
	if err := repo.CreateVersion(newVersion); err != nil {
		return nil, err
	}

	// Copiar módulos -> unidades -> recursos preservando stable_id.
	for _, modTree := range publishedTree.Modules {
		newModule := &domain.Module{
			CourseVersionID: newVersion.ID,
			StableID:        modTree.Module.StableID,
			Title:           modTree.Module.Title,
			Position:        modTree.Module.Position,
		}
		if err := repo.CreateModule(newModule); err != nil {
			return nil, err
		}
		for _, unitTree := range modTree.Units {
			newUnit := &domain.Unit{
				ModuleID: newModule.ID,
				StableID: unitTree.Unit.StableID,
				Title:    unitTree.Unit.Title,
				Position: unitTree.Unit.Position,
			}
			if err := repo.CreateUnit(newUnit); err != nil {
				return nil, err
			}
			for _, res := range unitTree.Resources {
				newRes := res // copia superficial: mismo stable_id, mismo contenido
				newRes.ID = ""
				newRes.UnitID = newUnit.ID
				if err := repo.CreateResource(&newRes); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := repo.UpdateCoursePublishedVersion(in.CourseID, course.PublishedVersionID, &newVersion.ID); err != nil {
		return nil, err
	}

	return newVersion, nil
}
