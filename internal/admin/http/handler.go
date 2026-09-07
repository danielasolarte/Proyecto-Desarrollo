package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
	authhttp "github.com/equipo-mooc/plataforma-mooc/internal/auth/http"
	"github.com/equipo-mooc/plataforma-mooc/internal/auth/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

type Handler struct {
	repo  domain.UserRepository
	store domain.SessionStore
}

func NewHandler(repo domain.UserRepository, store domain.SessionStore) *Handler {
	return &Handler{repo: repo, store: store}
}

type createTeacherRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type updateUserRequest struct {
	FullName *string        `json:"full_name"`
	Role     *domain.Role   `json:"role"`
	Status   *domain.Status `json:"status"`
}

func (h *Handler) ListUsers(c echo.Context) error {
	users, err := h.repo.ListUsers()
	if err != nil {
		return authhttpError(c, err)
	}
	return c.JSON(http.StatusOK, users)
}

func (h *Handler) CreateTeacher(c echo.Context) error {
	actor, _ := authctx.UserFromContext(c)
	var req createTeacherRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	u, err := usecase.CreateTeacher(h.repo, actor.ID, usecase.RegisterInput(req))
	if err != nil {
		return authhttpError(c, err)
	}
	return c.JSON(http.StatusCreated, u)
}

func (h *Handler) UpdateUser(c echo.Context) error {
	actor, _ := authctx.UserFromContext(c)
	var req updateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	u, err := usecase.UpdateUser(h.repo, usecase.UpdateUserInput{
		ActorID: actor.ID, UserID: c.Param("userID"), FullName: req.FullName, Role: req.Role, Status: req.Status,
	})
	if err != nil {
		return authhttpError(c, err)
	}
	return c.JSON(http.StatusOK, u)
}

func (h *Handler) DeleteUser(c echo.Context) error {
	actor, _ := authctx.UserFromContext(c)
	if err := usecase.DeleteUser(h.repo, actor.ID, c.Param("userID")); err != nil {
		return authhttpError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListUserSessions(c echo.Context) error {
	sessions, err := h.repo.ListSessionsByUser(c.Param("userID"))
	if err != nil {
		return authhttpError(c, err)
	}
	return c.JSON(http.StatusOK, sessions)
}

func (h *Handler) RevokeSession(c echo.Context) error {
	actor, _ := authctx.UserFromContext(c)
	if err := usecase.RevokeSession(h.repo, h.store, actor.ID, c.Param("sessionID")); err != nil {
		return authhttpError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListAuditLog(c echo.Context) error {
	logs, err := h.repo.ListAuditLog()
	if err != nil {
		return authhttpError(c, err)
	}
	return c.JSON(http.StatusOK, logs)
}

func RegisterRoutes(e *echo.Echo, h *Handler) {
	g := e.Group("/api/v1/admin", authctx.RequireRole(authctx.RoleAdmin))
	g.GET("/users", h.ListUsers)
	g.POST("/users/teachers", h.CreateTeacher)
	g.PATCH("/users/:userID", h.UpdateUser)
	g.DELETE("/users/:userID", h.DeleteUser)
	g.GET("/users/:userID/sessions", h.ListUserSessions)
	g.DELETE("/sessions/:sessionID", h.RevokeSession)
	g.GET("/audit-log", h.ListAuditLog)
}

func authhttpError(c echo.Context, err error) error {
	return authhttp.RespondErrorForAdmin(c, err)
}
