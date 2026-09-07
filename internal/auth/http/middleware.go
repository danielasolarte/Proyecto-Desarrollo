package http

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/auth/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

func AuthMiddleware(repo domain.UserRepository, store domain.SessionStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := bearerToken(c.Request().Header.Get("Authorization"))
			if token == "" {
				token = c.Request().Header.Get("X-Session-Token")
			}
			if token != "" {
				u, err := usecase.Authenticate(repo, store, token)
				if err == nil {
					ctx := authctx.ContextWithUser(c.Request().Context(), authctx.User{
						ID:   u.ID,
						Role: authctx.Role(u.Role),
					})
					c.SetRequest(c.Request().WithContext(ctx))
				}
			}
			return next(c)
		}
	}
}

func bearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
