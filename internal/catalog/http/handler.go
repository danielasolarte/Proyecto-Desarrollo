package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/catalog/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/catalog/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

type Handler struct {
	repo domain.CatalogRepository
}

func NewHandler(repo domain.CatalogRepository) *Handler {
	return &Handler{repo: repo}
}

func respondError(c echo.Context, err error) error {
	var code, message string
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrCourseNotPublished):
		code, message = apierror.CodeNotFound, "curso no encontrado"
	case errors.Is(err, domain.ErrAlreadyEnrolled):
		code, message = apierror.CodeConflict, "ya estas inscrito en este curso"
	case errors.Is(err, domain.ErrNotEnrolled):
		code, message = apierror.CodeConflict, "no estas inscrito en este curso"
	default:
		code, message = apierror.CodeInternal, "error interno"
	}
	return c.JSON(apierror.StatusFor(code), apierror.New(code, message))
}

// GetCatalog implementa GET /api/v1/catalog?q=&category=&cursor=&limit=
func (h *Handler) GetCatalog(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	result, err := usecase.SearchCatalog(h.repo, usecase.SearchInput{
		Query:    c.QueryParam("q"),
		Category: c.QueryParam("category"),
		Cursor:   c.QueryParam("cursor"),
		Limit:    limit,
	})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"courses":     result.Courses,
		"next_cursor": result.NextCursor,
	})
}

func currentStudent(c echo.Context) (string, error) {
	u, ok := authctx.UserFromContext(c)
	if !ok {
		return "", domain.ErrNotFound
	}
	return u.ID, nil
}

// Enroll implementa POST /api/v1/courses/:courseID/enrollments
func (h *Handler) Enroll(c echo.Context) error {
	studentID, err := currentStudent(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, apierror.New(apierror.CodeUnauthorized, "no autenticado"))
	}
	courseID := c.Param("courseID")
	e, err := usecase.Enroll(h.repo, studentID, courseID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, e)
}

// Withdraw implementa DELETE /api/v1/courses/:courseID/enrollments
func (h *Handler) Withdraw(c echo.Context) error {
	studentID, err := currentStudent(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, apierror.New(apierror.CodeUnauthorized, "no autenticado"))
	}
	courseID := c.Param("courseID")
	if err := usecase.Withdraw(h.repo, studentID, courseID); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListMyEnrollments implementa GET /api/v1/me/enrollments
func (h *Handler) ListMyEnrollments(c echo.Context) error {
	studentID, err := currentStudent(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, apierror.New(apierror.CodeUnauthorized, "no autenticado"))
	}
	list, err := usecase.ListMyEnrollments(h.repo, studentID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, list)
}
