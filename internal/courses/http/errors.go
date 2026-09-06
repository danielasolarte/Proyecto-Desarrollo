package http

import (
	"errors"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
)

// respondError traduce un error de dominio al formato uniforme de error
// de la API. Cualquier error no reconocido se trata como 500 y no se
// filtra su mensaje interno al cliente.
func respondError(c echo.Context, err error) error {
	var code, message string

	switch {
	case errors.Is(err, domain.ErrNotFound):
		code, message = apierror.CodeNotFound, "recurso no encontrado"
	case errors.Is(err, domain.ErrForbidden):
		code, message = apierror.CodeForbidden, "no tienes permiso sobre este recurso"
	case errors.Is(err, domain.ErrVersionNotDraft):
		code, message = apierror.CodeLocked, "esta version ya fue publicada y no se puede editar"
	case errors.Is(err, domain.ErrCourseHasNoPublished):
		code, message = apierror.CodeValidation, "el curso todavia no tiene ninguna version publicada"
	case errors.Is(err, domain.ErrCourseAlreadyHasDraft):
		code, message = apierror.CodeConflict, "el curso ya tiene un borrador de actualizacion en curso"
	case errors.Is(err, domain.ErrNotRichText):
		code, message = apierror.CodeValidation, "esta operacion solo aplica a recursos de tipo rich_text"
	case errors.Is(err, domain.ErrValidation):
		code, message = apierror.CodeValidation, err.Error()
	default:
		code, message = apierror.CodeInternal, "error interno"
	}

	return c.JSON(apierror.StatusFor(code), apierror.New(code, message))
}
