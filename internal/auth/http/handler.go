package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/auth/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

type Handler struct {
	repo   domain.UserRepository
	store  domain.SessionStore
	mailer domain.Mailer
}

func NewHandler(repo domain.UserRepository, store domain.SessionStore, mailer domain.Mailer) *Handler {
	return &Handler{repo: repo, store: store, mailer: mailer}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenRequest struct {
	Token string `json:"token"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	u, token, err := usecase.Register(h.repo, h.mailer, usecase.RegisterInput(req))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"user": u, "verification_token_dev": token})
}

func (h *Handler) BootstrapAdmin(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	u, err := usecase.BootstrapAdmin(h.repo, usecase.RegisterInput(req))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, u)
}

func (h *Handler) VerifyEmail(c echo.Context) error {
	var req tokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	u, err := usecase.VerifyEmail(h.repo, req.Token)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, u)
}

func (h *Handler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	result, err := usecase.Login(h.repo, h.store, usecase.LoginInput(req))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Logout(c echo.Context) error {
	token := bearerToken(c.Request().Header.Get("Authorization"))
	if token == "" {
		token = c.Request().Header.Get("X-Session-Token")
	}
	if token == "" {
		return respondError(c, domain.ErrUnauthorized)
	}
	if err := usecase.Logout(h.repo, h.store, token); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) Me(c echo.Context) error {
	u, ok := authctx.UserFromContext(c)
	if !ok {
		return respondError(c, domain.ErrUnauthorized)
	}
	user, err := h.repo.FindUserByID(u.ID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, user)
}

func (h *Handler) ForgotPassword(c echo.Context) error {
	var req forgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	if err := usecase.ForgotPassword(h.repo, h.mailer, req.Email); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusAccepted)
}

func (h *Handler) ResetPassword(c echo.Context) error {
	var req resetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}
	if err := usecase.ResetPassword(h.repo, req.Token, req.NewPassword); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func RegisterRoutes(e *echo.Echo, h *Handler) {
	g := e.Group("/api/v1")
	g.POST("/auth/register", h.Register)
	g.POST("/auth/bootstrap-admin", h.BootstrapAdmin)
	g.POST("/auth/verify-email", h.VerifyEmail)
	g.POST("/auth/login", h.Login)
	g.POST("/auth/password/forgot", h.ForgotPassword)
	g.POST("/auth/password/reset", h.ResetPassword)

	authenticated := g.Group("", authctx.RequireRole(authctx.RoleStudent, authctx.RoleTeacher, authctx.RoleAdmin))
	authenticated.GET("/me", h.Me)
	authenticated.POST("/auth/logout", h.Logout)
}
