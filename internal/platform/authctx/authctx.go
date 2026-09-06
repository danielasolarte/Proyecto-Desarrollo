// Package authctx es un SHIM TEMPORAL de autenticación.
//
// El módulo de identidad y sesiones (Persona A, internal/auth) todavía no
// existe. Mientras tanto, este paquete lee el usuario autenticado de dos
// headers (X-User-Id, X-User-Role) para poder desarrollar y probar el
// módulo de cursos/catálogo de forma independiente.
//
// CUANDO EL MÓDULO DE AUTH ESTÉ LISTO: reemplazar el middleware Fake por
// el middleware real que valida la sesión contra Redis y llena el mismo
// contexto (CurrentUser), y borrar este archivo. Los handlers de
// courses/catalog no deberían necesitar ningún cambio porque dependen
// solo de la función UserFromContext, no de cómo se llena.
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

// FakeAuthMiddleware lee X-User-Id y X-User-Role de la petición y los mete
// al contexto. Sirve solo para desarrollo local mientras no existe el
// módulo real de sesiones.
func FakeAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := c.Request().Header.Get("X-User-Id")
			role := c.Request().Header.Get("X-User-Role")
			if userID != "" && role != "" {
				ctx := context.WithValue(c.Request().Context(), userContextKey, User{
					ID:   userID,
					Role: Role(role),
				})
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}

// UserFromContext devuelve el usuario autenticado de la petición actual.
// El segundo valor es false si no hay usuario (no autenticado).
func UserFromContext(c echo.Context) (User, bool) {
	u, ok := c.Request().Context().Value(userContextKey).(User)
	return u, ok
}

// RequireRole es un middleware que exige que el usuario autenticado tenga
// uno de los roles dados. Debe ir después de FakeAuthMiddleware (o del
// middleware real de auth) en la cadena.
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
