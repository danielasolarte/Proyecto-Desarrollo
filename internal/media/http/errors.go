package http

import (
	"errors"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
)

// respondError traduce un error de dominio al formato uniforme de error
// de la API, igual que internal/courses/http/errors.go. Cualquier error no
// reconocido se trata como 500 y no se filtra su mensaje interno.
func respondError(c echo.Context, err error) error {
	var code, message string

	switch {
	case errors.Is(err, domain.ErrNotFound):
		code, message = apierror.CodeNotFound, "recurso multimedia no encontrado"
	case errors.Is(err, domain.ErrForbidden):
		code, message = apierror.CodeForbidden, "no tienes permiso sobre esta carga"
	case errors.Is(err, domain.ErrUploadNotActive):
		code, message = apierror.CodeConflict, "esta sesion de carga ya no esta activa"
	case errors.Is(err, domain.ErrUploadExpired):
		code, message = apierror.CodeValidation, "la sesion de carga expiro, inicia una nueva"
	case errors.Is(err, domain.ErrChecksumMismatch):
		code, message = apierror.CodeValidation, "el archivo recibido no coincide con el checksum declarado"
	case errors.Is(err, domain.ErrMimeNotAllowed):
		code, message = apierror.CodeValidation, "el tipo de archivo detectado no esta permitido"
	case errors.Is(err, domain.ErrJobAlreadyTerminal):
		code, message = apierror.CodeConflict, "este trabajo ya esta en un estado final"
	case errors.Is(err, domain.ErrValidation):
		code, message = apierror.CodeValidation, err.Error()
	default:
		code, message = apierror.CodeInternal, "error interno"
	}

	return c.JSON(apierror.StatusFor(code), apierror.New(code, message))
}
