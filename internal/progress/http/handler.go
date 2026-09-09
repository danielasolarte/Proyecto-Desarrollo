package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	catalogDomain "github.com/equipo-mooc/plataforma-mooc/internal/catalog/domain"
	coursesDomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
	progressDomain "github.com/equipo-mooc/plataforma-mooc/internal/progress/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/progress/usecase"
)

type Handler struct {
	service     *usecase.Service
	courseRepo  coursesDomain.CourseRepository
	catalogRepo catalogDomain.CatalogRepository
}

func NewHandler(
	service *usecase.Service,
	courseRepo coursesDomain.CourseRepository,
	catalogRepo catalogDomain.CatalogRepository,
) *Handler {
	return &Handler{
		service:     service,
		courseRepo:  courseRepo,
		catalogRepo: catalogRepo,
	}
}

func currentStudent(c echo.Context) (authctx.User, uuid.UUID, error) {
	user, ok := authctx.UserFromContext(c)
	if !ok {
		return authctx.User{}, uuid.Nil, progressDomain.ErrForbidden
	}

	studentID, err := uuid.Parse(user.ID)
	if err != nil {
		return authctx.User{}, uuid.Nil, progressDomain.ErrForbidden
	}

	return user, studentID, nil
}

func (h *Handler) courseForResource(
	resourceID string,
) (*coursesDomain.Course, *coursesDomain.Resource, error) {

	resource, err := h.courseRepo.FindResourceByID(resourceID)
	if err != nil {
		return nil, nil, err
	}

	unit, err := h.courseRepo.FindUnitByID(resource.UnitID)
	if err != nil {
		return nil, nil, err
	}

	module, err := h.courseRepo.FindModuleByID(unit.ModuleID)
	if err != nil {
		return nil, nil, err
	}

	version, err := h.courseRepo.FindVersionByID(module.CourseVersionID)
	if err != nil {
		return nil, nil, err
	}

	course, err := h.courseRepo.FindCourseByID(version.CourseID)
	if err != nil {
		return nil, nil, err
	}

	return course, resource, nil
}

func (h *Handler) activeEnrollment(
	studentID string,
	courseID string,
) (*catalogDomain.Enrollment, error) {

	enrollment, err := h.catalogRepo.FindEnrollment(studentID, courseID)
	if err != nil {
		return nil, progressDomain.ErrEnrollmentNeeded
	}

	if enrollment.Status != catalogDomain.EnrollmentActive {
		return nil, progressDomain.ErrEnrollmentNeeded
	}

	return enrollment, nil
}

func respondError(c echo.Context, err error) error {
	switch err {
	case progressDomain.ErrProgressNotFound:
		return c.JSON(
			http.StatusNotFound,
			apierror.New(apierror.CodeNotFound, err.Error()),
		)

	case progressDomain.ErrInvalidHeartbeat,
		progressDomain.ErrInvalidProgress:
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, err.Error()),
		)

	case progressDomain.ErrForbidden,
		progressDomain.ErrEnrollmentNeeded:
		return c.JSON(
			http.StatusForbidden,
			apierror.New(apierror.CodeForbidden, err.Error()),
		)

	default:
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "error interno"),
		)
	}
}

func (h *Handler) OpenResource(c echo.Context) error {
	user, studentID, err := currentStudent(c)
	if err != nil {
		return respondError(c, err)
	}

	resourceIDString := c.Param("resourceID")

	resourceID, err := uuid.Parse(resourceIDString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "resource_id invalido"),
		)
	}

	course, resource, err := h.courseForResource(resourceIDString)
	if err != nil {
		return respondError(c, err)
	}

	enrollment, err := h.activeEnrollment(user.ID, course.ID)
	if err != nil {
		return respondError(c, err)
	}

	enrollmentID, err := uuid.Parse(enrollment.ID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "enrollment_id invalido"),
		)
	}

	stableID, err := uuid.Parse(resource.StableID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "stable_id invalido"),
		)
	}

	progress, err := h.service.OpenResource(
		c.Request().Context(),
		studentID,
		enrollmentID,
		resourceID,
		stableID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, progress)
}

type heartbeatRequest struct {
	PositionSeconds *int       `json:"position_seconds"`
	ClientTimestamp *time.Time `json:"client_timestamp"`
}

func (h *Handler) Heartbeat(c echo.Context) error {
	user, studentID, err := currentStudent(c)
	if err != nil {
		return respondError(c, err)
	}

	resourceIDString := c.Param("resourceID")

	resourceID, err := uuid.Parse(resourceIDString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "resource_id invalido"),
		)
	}

	course, resource, err := h.courseForResource(resourceIDString)
	if err != nil {
		return respondError(c, err)
	}

	enrollment, err := h.activeEnrollment(user.ID, course.ID)
	if err != nil {
		return respondError(c, err)
	}

	enrollmentID, err := uuid.Parse(enrollment.ID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "enrollment_id invalido"),
		)
	}

	stableID, err := uuid.Parse(resource.StableID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "stable_id invalido"),
		)
	}

	var req heartbeatRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "cuerpo invalido"),
		)
	}

	progress, err := h.service.Heartbeat(
		c.Request().Context(),
		studentID,
		enrollmentID,
		resourceID,
		stableID,
		req.PositionSeconds,
		req.ClientTimestamp,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, progress)
}

func (h *Handler) CompleteResource(c echo.Context) error {
	user, studentID, err := currentStudent(c)
	if err != nil {
		return respondError(c, err)
	}

	resourceIDString := c.Param("resourceID")

	resourceID, err := uuid.Parse(resourceIDString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "resource_id invalido"),
		)
	}

	course, resource, err := h.courseForResource(resourceIDString)
	if err != nil {
		return respondError(c, err)
	}

	enrollment, err := h.activeEnrollment(user.ID, course.ID)
	if err != nil {
		return respondError(c, err)
	}

	enrollmentID, err := uuid.Parse(enrollment.ID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "enrollment_id invalido"),
		)
	}

	stableID, err := uuid.Parse(resource.StableID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "stable_id invalido"),
		)
	}

	progress, err := h.service.CompleteResource(
		c.Request().Context(),
		studentID,
		enrollmentID,
		resourceID,
		stableID,
	)
	if err != nil {
		return respondError(c, err)
	}

	courseID, err := uuid.Parse(course.ID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "course_id invalido"),
		)
	}

	courseProgress, err := h.service.RecalculateCourseProgress(
		c.Request().Context(),
		studentID,
		enrollmentID,
		courseID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"resource_progress": progress,
		"course_progress":   courseProgress,
	})
}

func (h *Handler) GetCourseProgress(c echo.Context) error {
	user, studentID, err := currentStudent(c)
	if err != nil {
		return respondError(c, err)
	}

	courseIDString := c.Param("courseID")

	courseID, err := uuid.Parse(courseIDString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "course_id invalido"),
		)
	}

	enrollment, err := h.activeEnrollment(user.ID, courseIDString)
	if err != nil {
		return respondError(c, err)
	}

	enrollmentID, err := uuid.Parse(enrollment.ID)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apierror.New(apierror.CodeInternal, "enrollment_id invalido"),
		)
	}

	progress, err := h.service.GetCourseProgress(
		c.Request().Context(),
		studentID,
		enrollmentID,
		courseID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, progress)
}

