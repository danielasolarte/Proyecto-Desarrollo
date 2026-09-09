package http

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	badgeDomain "github.com/equipo-mooc/plataforma-mooc/internal/badges/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/badges/usecase"
	coursesDomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

type Handler struct {
	service    *usecase.Service
	courseRepo coursesDomain.CourseRepository
}

func NewHandler(
	service *usecase.Service,
	courseRepo coursesDomain.CourseRepository,
) *Handler {
	return &Handler{
		service:    service,
		courseRepo: courseRepo,
	}
}

func respondError(c echo.Context, err error) error {
	switch err {

	case badgeDomain.ErrBadgeNotFound,
		badgeDomain.ErrIssuanceNotFound:
		return c.JSON(
			http.StatusNotFound,
			apierror.New(apierror.CodeNotFound, err.Error()),
		)

	case badgeDomain.ErrCourseNotApproved,
		badgeDomain.ErrBadgeInactive:
		return c.JSON(
			http.StatusConflict,
			apierror.New(apierror.CodeConflict, err.Error()),
		)

	case badgeDomain.ErrForbidden:
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

type createBadgeRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	ImageURL    *string `json:"image_url"`
}

func (h *Handler) CreateBadge(c echo.Context) error {
	user, ok := authctx.UserFromContext(c)
	if !ok {
		return respondError(c, badgeDomain.ErrForbidden)
	}

	courseIDString := c.Param("courseID")

	courseID, err := uuid.Parse(courseIDString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "course_id invalido"),
		)
	}

	course, err := h.courseRepo.FindCourseByID(courseIDString)
	if err != nil {
		return c.JSON(
			http.StatusNotFound,
			apierror.New(apierror.CodeNotFound, "course not found"),
		)
	}

	if user.Role != authctx.RoleAdmin && course.TeacherID != user.ID {
		return respondError(c, badgeDomain.ErrForbidden)
	}

	var req createBadgeRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "cuerpo invalido"),
		)
	}

	if req.Name == "" {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "name es requerido"),
		)
	}

	badge, err := h.service.CreateBadge(
		c.Request().Context(),
		courseID,
		req.Name,
		req.Description,
		req.ImageURL,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusCreated, badge)
}

func (h *Handler) VerifyBadge(c echo.Context) error {
	codeString := c.Param("verificationCode")

	code, err := uuid.Parse(codeString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "verification_code invalido"),
		)
	}

	result, err := h.service.Verify(
		c.Request().Context(),
		code,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusOK, result)
}

func (h *Handler) RevokeIssuance(c echo.Context) error {
	issuanceIDString := c.Param("issuanceID")

	issuanceID, err := uuid.Parse(issuanceIDString)
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apierror.New(apierror.CodeValidation, "issuance_id invalido"),
		)
	}

	if err := h.service.Revoke(
		c.Request().Context(),
		issuanceID,
	); err != nil {
		return respondError(c, err)
	}

	return c.JSON(
		http.StatusOK,
		map[string]any{
			"revoked": true,
		},
	)
}

