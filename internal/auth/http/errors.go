package http

import (
	"errors"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
)

func respondError(c echo.Context, err error) error {
	var code, message string
	switch {
	case errors.Is(err, domain.ErrEmailAlreadyTaken):
		code, message = apierror.CodeConflict, "email ya registrado"
	case errors.Is(err, domain.ErrInvalidCredentials):
		code, message = apierror.CodeUnauthorized, "credenciales invalidas"
	case errors.Is(err, domain.ErrUnauthorized):
		code, message = apierror.CodeUnauthorized, "no autenticado"
	case errors.Is(err, domain.ErrForbidden):
		code, message = apierror.CodeForbidden, "no autorizado"
	case errors.Is(err, domain.ErrInactiveUser):
		code, message = apierror.CodeForbidden, "usuario inactivo"
	case errors.Is(err, domain.ErrLastActiveAdmin):
		code, message = apierror.CodeConflict, "no se puede modificar el ultimo administrador activo"
	case errors.Is(err, domain.ErrAdminAlreadyExists):
		code, message = apierror.CodeConflict, "ya existe un administrador activo"
	case errors.Is(err, domain.ErrCannotSelfDowngrade):
		code, message = apierror.CodeConflict, "no puedes degradar o suspender tu propia cuenta admin"
	case errors.Is(err, domain.ErrInvalidToken):
		code, message = apierror.CodeValidation, "token invalido o expirado"
	case errors.Is(err, domain.ErrValidation):
		code, message = apierror.CodeValidation, err.Error()
	case errors.Is(err, domain.ErrNotFound):
		code, message = apierror.CodeNotFound, "recurso no encontrado"
	default:
		code, message = apierror.CodeInternal, "error interno"
	}
	return c.JSON(apierror.StatusFor(code), apierror.New(code, message))
}

func RespondErrorForAdmin(c echo.Context, err error) error {
	return respondError(c, err)
}
