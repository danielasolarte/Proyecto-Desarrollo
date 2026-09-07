package authctx

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

type User struct {
	ID   string
	Role Role
}

type contextKey string

const userContextKey contextKey = "current_user"

func ContextWithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func FakeAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := c.Request().Header.Get("X-User-Id")
			role := c.Request().Header.Get("X-User-Role")
			if userID != "" && role != "" {
				ctx := ContextWithUser(c.Request().Context(), User{ID: userID, Role: Role(role)})
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}

func UserFromContext(c echo.Context) (User, bool) {
	u, ok := c.Request().Context().Value(userContextKey).(User)
	return u, ok
}

func RequireRole(roles ...Role) echo.MiddlewareFunc {
	allowed := make(map[Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			u, ok := UserFromContext(c)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"error": map[string]string{"code": "unauthorized", "message": "no autenticado"},
				})
			}
			if !allowed[u.Role] {
				return c.JSON(http.StatusForbidden, map[string]any{
					"error": map[string]string{"code": "forbidden", "message": "rol no autorizado para esta operacion"},
				})
			}
			return next(c)
		}
	}
}
