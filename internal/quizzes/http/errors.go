package http

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/quizzes/domain"
)

func respondError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrQuizNotFound),
		errors.Is(err, domain.ErrQuestionNotFound),
		errors.Is(err, domain.ErrOptionNotFound),
		errors.Is(err, domain.ErrAttemptNotFound):

		return c.JSON(
			http.StatusNotFound,
			apierror.New(apierror.CodeNotFound, err.Error()),
		)

	case errors.Is(err, domain.ErrForbidden):
		return c.JSON(
			http.StatusForbidden,
			apierror.New(apierror.CodeForbidden, err.Error()),
		)

	case errors.Is(err, domain.ErrMaxAttemptsReached):
		return c.JSON(
			http.StatusConflict,
			apierror.New(apierror.CodeConflict, err.Error()),
		)

	case errors.Is(err, domain.ErrAttemptExpired):
		return c.JSON(
			http.StatusConflict,
			apierror.New(apierror.CodeConflict, err.Error()),
		)

	case errors.Is(err, domain.ErrAttemptAlreadySubmitted):
		return c.JSON(
			http.StatusConflict,
			apierror.New(apierror.CodeConflict, err.Error()),
		)

	case errors.Is(err, domain.ErrInvalidAnswer),
		errors.Is(err, domain.ErrInvalidQuiz):

		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, err.Error()),
		)

	case errors.Is(err, domain.ErrIdempotencyConflict):
		return c.JSON(
			http.StatusConflict,
			apierror.New(apierror.CodeConflict, err.Error()),
		)

	case errors.Is(err, domain.ErrUnauthorized):
		return c.JSON(
			http.StatusUnauthorized,
			apierror.New(apierror.CodeUnauthorized, "no autenticado"),
		)

	default:
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "error interno"),
		)
	}
}

