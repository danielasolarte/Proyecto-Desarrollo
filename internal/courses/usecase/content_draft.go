package usecase

import (
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

// SaveContentDraft implementa el autosave del editor de bloques: guarda el
// contenido en Markdown extendido en content_draft_markdown SIN tocar el
// contenido publicado (content_markdown). Se puede llamar repetidas veces
// mientras el profesor escribe.
func SaveContentDraft(repo domain.CourseRepository, resourceID string, markdown string) (*domain.Resource, error) {
	res, err := resourceVersionGuard(repo, resourceID)
	if err != nil {
		return nil, err
	}
	if res.Type != domain.ResourceRichText {
		return nil, domain.ErrNotRichText
	}
	now := time.Now()
	res.ContentDraftMarkdown = &markdown
	res.DraftSavedAt = &now
	if err := repo.UpdateResource(res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetContentDraft recupera el último borrador autosaved (recuperación
// tras cerrar el navegador o perder la conexión, sin perder cambios).
func GetContentDraft(repo domain.CourseRepository, resourceID string) (*domain.Resource, error) {
	res, err := repo.FindResourceByID(resourceID)
	if err != nil {
		return nil, err
	}
	if res.Type != domain.ResourceRichText {
		return nil, domain.ErrNotRichText
	}
	return res, nil
}

// PublishContentDraft mueve el borrador autosaved a contenido "oficial"
// dentro de la versión en edición. Esto NO publica la versión del curso:
// solo confirma que ese recurso, dentro del draft, ya tiene el contenido
// que se quiere llevar a la próxima publicación.
func PublishContentDraft(repo domain.CourseRepository, resourceID string) (*domain.Resource, error) {
	res, err := resourceVersionGuard(repo, resourceID)
	if err != nil {
		return nil, err
	}
	if res.Type != domain.ResourceRichText {
		return nil, domain.ErrNotRichText
	}
	if res.ContentDraftMarkdown != nil {
		res.ContentMarkdown = res.ContentDraftMarkdown
	}
	if err := repo.UpdateResource(res); err != nil {
		return nil, err
	}
	return res, nil
}
