// Package apierror define el formato uniforme de error de la API,
// acordado por todo el equipo: {"error": {"code": "...", "message": "..."}}.
package apierror

import "net/http"

// APIError es el cuerpo de error que devuelve cualquier endpoint de la API.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Envelope envuelve un APIError bajo la clave "error", que es lo que
// realmente se serializa en la respuesta HTTP.
type Envelope struct {
	Error APIError `json:"error"`
}

func New(code, message string) Envelope {
	return Envelope{Error: APIError{Code: code, Message: message}}
}

// Códigos de error reutilizables entre módulos. Cada módulo puede definir
// códigos adicionales propios (p.ej. "email_already_taken" en auth).
const (
	CodeValidation   = "validation_error"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeForbidden    = "forbidden"
	CodeUnauthorized = "unauthorized"
	CodeInternal     = "internal_error"
	CodeLocked       = "locked"
)

// StatusFor mapea un código de error al status HTTP correspondiente,
// para no repetir ese mapeo en cada handler.
func StatusFor(code string) int {
	switch code {
	case CodeValidation:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeForbidden:
		return http.StatusForbidden
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeLocked:
		return http.StatusLocked
	default:
		return http.StatusInternalServerError
	}
}
