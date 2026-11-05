package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

// Sin caché: Get nunca encuentra la sesión, así Authenticate siempre la
// valida en Postgres; Save y Delete no fallan.
func TestPostgresOnlySessionStore(t *testing.T) {
	s := NewPostgresOnlySessionStore()
	if err := s.Save("hash", domain.User{ID: "u1"}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if u, err := s.Get("hash"); u != nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get debía responder ErrNotFound, llegó %v, %v", u, err)
	}
	if err := s.Delete("hash"); err != nil {
		t.Fatal(err)
	}
}
