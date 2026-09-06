// Package pagination implementa paginación por cursor (keyset), como pide
// la sección 7 del enunciado ("cursores, ETag e Idempotency-Key").
//
// El cursor es simplemente el ID (UUID) del último elemento visto,
// codificado en base64 para que sea un valor opaco desde el punto de
// vista del cliente.
package pagination

import "encoding/base64"

const DefaultLimit = 20
const MaxLimit = 100

func EncodeCursor(lastID string) string {
	if lastID == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(lastID))
}

func DecodeCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// NormalizeLimit aplica el límite por defecto y el tope máximo.
func NormalizeLimit(requested int) int {
	if requested <= 0 {
		return DefaultLimit
	}
	if requested > MaxLimit {
		return MaxLimit
	}
	return requested
}
