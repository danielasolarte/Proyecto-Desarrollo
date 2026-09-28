// Package http expone el módulo de cursos como endpoints REST bajo
// /api/v1, usando Echo. Esta capa solo hace: leer la petición, verificar
// autenticación/propiedad, llamar al usecase correspondiente, y traducir
// el resultado a JSON. Ninguna regla de negocio vive aquí.
package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/courses/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

type Handler struct {
	repo domain.CourseRepository
}

func NewHandler(repo domain.CourseRepository) *Handler {
	return &Handler{repo: repo}
}

// ---------- helpers de autorización ----------

func (h *Handler) courseForVersion(versionID string) (*domain.Course, *domain.CourseVersion, error) {
	v, err := h.repo.FindVersionByID(versionID)
	if err != nil {
		return nil, nil, err
	}
	c, err := h.repo.FindCourseByID(v.CourseID)
	if err != nil {
		return nil, nil, err
	}
	return c, v, nil
}

func (h *Handler) courseForModule(moduleID string) (*domain.Course, *domain.Module, error) {
	m, err := h.repo.FindModuleByID(moduleID)
	if err != nil {
		return nil, nil, err
	}
	c, _, err := h.courseForVersion(m.CourseVersionID)
	if err != nil {
		return nil, nil, err
	}
	return c, m, nil
}

func (h *Handler) courseForUnit(unitID string) (*domain.Course, *domain.Unit, error) {
	u, err := h.repo.FindUnitByID(unitID)
	if err != nil {
		return nil, nil, err
	}
	c, _, err := h.courseForModule(u.ModuleID)
	if err != nil {
		return nil, nil, err
	}
	return c, u, nil
}

func (h *Handler) courseForResource(resourceID string) (*domain.Course, *domain.Resource, error) {
	res, err := h.repo.FindResourceByID(resourceID)
	if err != nil {
		return nil, nil, err
	}
	c, _, err := h.courseForUnit(res.UnitID)
	if err != nil {
		return nil, nil, err
	}
	return c, res, nil
}

func currentUser(c echo.Context) (authctx.User, error) {
	u, ok := authctx.UserFromContext(c)
	if !ok {
		return authctx.User{}, domain.ErrForbidden
	}
	return u, nil
}

// ---------- cursos ----------

type createCourseRequest struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Category string `json:"category"`
}

func (h *Handler) CreateCourse(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}

	var req createCourseRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}

	course, version, err := usecase.CreateCourse(h.repo, usecase.CreateCourseInput{
		TeacherID: user.ID, Title: req.Title, Summary: req.Summary, Category: req.Category,
	})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"course": course, "version": version})
}

func (h *Handler) GetCourse(c echo.Context) error {
	course, err := h.repo.FindCourseByID(c.Param("courseID"))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, course)
}

type createUpdateDraftRequest struct {
	ChangeType string `json:"change_type"`
}

func (h *Handler) CreateUpdateDraft(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	courseID := c.Param("courseID")
	course, err := h.repo.FindCourseByID(courseID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req createUpdateDraftRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}

	version, err := usecase.CreateUpdateDraft(h.repo, usecase.CreateUpdateDraftInput{
		CourseID: courseID, ChangeType: domain.ChangeType(req.ChangeType),
	})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, version)
}

func (h *Handler) PublishCourse(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	courseID := c.Param("courseID")
	course, err := h.repo.FindCourseByID(courseID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	version, err := usecase.PublishVersion(h.repo, courseID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, version)
}

func (h *Handler) PreviewCourse(c echo.Context) error {
	courseID := c.Param("courseID")
	tree, problems, err := usecase.Preview(h.repo, courseID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"tree": tree, "problems": problems})
}

// ---------- versiones ----------

type updateVersionRequest struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Category string `json:"category"`
}

func (h *Handler) UpdateVersion(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	versionID := c.Param("versionID")
	course, version, err := h.courseForVersion(versionID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}
	if version.Status != domain.VersionStatusDraft {
		return respondError(c, domain.ErrVersionNotDraft)
	}

	var req updateVersionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	version.Title, version.Summary, version.Category = req.Title, req.Summary, req.Category
	if err := h.repo.UpdateVersionMetadata(version); err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, version)
}

// ---------- módulos ----------

type moduleRequest struct {
	Title    string `json:"title"`
	Position int    `json:"position"`
}

func (h *Handler) CreateModule(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	versionID := c.Param("versionID")
	course, _, err := h.courseForVersion(versionID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req moduleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	m, err := usecase.CreateModule(h.repo, usecase.CreateModuleInput{VersionID: versionID, Title: req.Title, Position: req.Position})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, m)
}

func (h *Handler) UpdateModule(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	moduleID := c.Param("moduleID")
	course, _, err := h.courseForModule(moduleID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req moduleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	m, err := usecase.UpdateModule(h.repo, usecase.UpdateModuleInput{ModuleID: moduleID, Title: req.Title, Position: req.Position})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, m)
}

func (h *Handler) DeleteModule(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	moduleID := c.Param("moduleID")
	course, _, err := h.courseForModule(moduleID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}
	if err := usecase.DeleteModule(h.repo, moduleID); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------- unidades ----------

type unitRequest struct {
	Title    string `json:"title"`
	Position int    `json:"position"`
}

func (h *Handler) CreateUnit(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	moduleID := c.Param("moduleID")
	course, _, err := h.courseForModule(moduleID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req unitRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	u, err := usecase.CreateUnit(h.repo, usecase.CreateUnitInput{ModuleID: moduleID, Title: req.Title, Position: req.Position})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, u)
}

func (h *Handler) UpdateUnit(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	unitID := c.Param("unitID")
	course, _, err := h.courseForUnit(unitID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req unitRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	u, err := usecase.UpdateUnit(h.repo, usecase.UpdateUnitInput{UnitID: unitID, Title: req.Title, Position: req.Position})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, u)
}

func (h *Handler) DeleteUnit(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	unitID := c.Param("unitID")
	course, _, err := h.courseForUnit(unitID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}
	if err := usecase.DeleteUnit(h.repo, unitID); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------- recursos ----------

type resourceRequest struct {
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Position     int     `json:"position"`
	Visible      bool    `json:"visible"`
	Required     bool    `json:"required"`
	Downloadable bool    `json:"downloadable"`
	ExternalURL  *string `json:"external_url"`
}

func (h *Handler) CreateResource(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	unitID := c.Param("unitID")
	course, _, err := h.courseForUnit(unitID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req resourceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	res, err := usecase.CreateResource(h.repo, usecase.CreateResourceInput{
		UnitID: unitID, Type: domain.ResourceType(req.Type), Title: req.Title, Position: req.Position,
		Visible: req.Visible, Required: req.Required, Downloadable: req.Downloadable, ExternalURL: req.ExternalURL,
	})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, res)
}

func (h *Handler) UpdateResource(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	course, _, err := h.courseForResource(resourceID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req resourceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	res, err := usecase.UpdateResource(h.repo, usecase.UpdateResourceInput{
		ResourceID: resourceID, Title: req.Title, Position: req.Position,
		Visible: req.Visible, Required: req.Required, Downloadable: req.Downloadable, ExternalURL: req.ExternalURL,
	})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) DeleteResource(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	course, _, err := h.courseForResource(resourceID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}
	if err := usecase.DeleteResource(h.repo, resourceID); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------- editor: autosave / recuperación / publicar contenido ----------

type saveDraftRequest struct {
	Markdown string `json:"markdown"`
}

func (h *Handler) SaveContentDraft(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	course, _, err := h.courseForResource(resourceID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	var req saveDraftRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	res, err := usecase.SaveContentDraft(h.repo, resourceID, req.Markdown)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) GetContentDraft(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	course, _, err := h.courseForResource(resourceID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	res, err := usecase.GetContentDraft(h.repo, resourceID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) PublishContentDraft(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	course, _, err := h.courseForResource(resourceID)
	if err != nil {
		return respondError(c, err)
	}
	if err := usecase.EnsureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}

	res, err := usecase.PublishContentDraft(h.repo, resourceID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
