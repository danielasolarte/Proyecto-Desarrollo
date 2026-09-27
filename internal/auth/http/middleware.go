package http

import (
	"net/http"
	"os"
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
			source := ""
			if token == "" {
				token = c.Request().Header.Get("X-Session-Token")
				if token != "" {
					source = "header"
				}
			} else {
				source = "bearer"
			}
			if token == "" {
				if cookie, err := c.Cookie(sessionCookieName()); err == nil {
					token = strings.TrimSpace(cookie.Value)
					if token != "" {
						source = "cookie"
					}
				}
			}
			if token != "" {
				u, err := usecase.Authenticate(repo, store, token)
				if err == nil {
					ctx := authctx.ContextWithUser(c.Request().Context(), authctx.User{
						ID:   u.ID,
						Role: authctx.Role(u.Role),
					})
					c.SetRequest(c.Request().WithContext(ctx))
					c.Set("auth_source", source)
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

func CSRFMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !boolEnv("CSRF_ENABLED", false) || csrfSafeMethod(c.Request().Method) {
				return next(c)
			}
			if c.Get("auth_source") != "cookie" {
				return next(c)
			}

			cookie, err := c.Cookie(csrfCookieName())
			if err != nil || cookie.Value == "" {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "csrf_required", "message": "token CSRF requerido"})
			}

			headerName := os.Getenv("CSRF_HEADER_NAME")
			if headerName == "" {
				headerName = "X-CSRF-Token"
			}
			if c.Request().Header.Get(headerName) != cookie.Value {
				return c.JSON(http.StatusForbidden, map[string]string{"code": "csrf_invalid", "message": "token CSRF invalido"})
			}

			return next(c)
		}
	}
}

func csrfSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func sessionCookieName() string {
	if value := os.Getenv("SESSION_COOKIE_NAME"); value != "" {
		return value
	}
	return "mooc_session"
}

func csrfCookieName() string {
	if value := os.Getenv("CSRF_COOKIE_NAME"); value != "" {
		return value
	}
	return "mooc_csrf"
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
