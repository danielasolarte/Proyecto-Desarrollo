package usecase

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

func TestEnsureBootstrapAdminCreatesFirstAdmin(t *testing.T) {
	repo := newMemoryRepo()
	res, err := EnsureBootstrapAdmin(repo, BootstrapAdminInput{Email: " Admin@Mooc.Test ", Password: "clave-segura-123"})
	if err != nil || res != BootstrapAdminCreated {
		t.Fatalf("resultado %q, err %v", res, err)
	}
	u, err := repo.FindUserByEmail("admin@mooc.test")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != domain.RoleAdmin || u.Status != domain.StatusActive || u.EmailVerifiedAt == nil || u.FullName == "" {
		t.Errorf("cuenta inesperada: %+v", u)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("clave-segura-123")) != nil {
		t.Error("la clave no quedó guardada como hash bcrypt")
	}
}

func TestEnsureBootstrapAdminDoesNothingWhenAdminExists(t *testing.T) {
	repo := newMemoryRepo()
	if _, err := EnsureBootstrapAdmin(repo, BootstrapAdminInput{Email: "a@mooc.test", Password: "clave-segura-123"}); err != nil {
		t.Fatal(err)
	}
	res, err := EnsureBootstrapAdmin(repo, BootstrapAdminInput{Email: "otro@mooc.test", Password: "otra-clave-456"})
	if err != nil || res != BootstrapAdminExists {
		t.Fatalf("resultado %q, err %v", res, err)
	}
	if _, err := repo.FindUserByEmail("otro@mooc.test"); !errors.Is(err, domain.ErrNotFound) {
		t.Error("no debía crear un segundo administrador")
	}
}

func TestEnsureBootstrapAdminDisabledWithoutConfiguration(t *testing.T) {
	for _, in := range []BootstrapAdminInput{{}, {Email: "a@mooc.test"}, {Password: "clave-segura-123"}} {
		res, err := EnsureBootstrapAdmin(newMemoryRepo(), in)
		if err != nil || res != BootstrapDisabled {
			t.Errorf("%+v: resultado %q, err %v", in, res, err)
		}
	}
}

// Si alguien se registró antes con el correo del administrador, no se
// promueve esa cuenta.
func TestEnsureBootstrapAdminRefusesExistingNonAdminEmail(t *testing.T) {
	repo := newMemoryRepo()
	_ = repo.CreateUser(&domain.User{Email: "admin@mooc.test", Role: domain.RoleStudent, Status: domain.StatusPendingVerification})
	if _, err := EnsureBootstrapAdmin(repo, BootstrapAdminInput{Email: "admin@mooc.test", Password: "clave-segura-123"}); !errors.Is(err, ErrBootstrapEmailTaken) {
		t.Fatalf("se esperaba ErrBootstrapEmailTaken, llegó %v", err)
	}
	u, _ := repo.FindUserByEmail("admin@mooc.test")
	if u.Role != domain.RoleStudent {
		t.Error("la cuenta existente no debía cambiar de rol")
	}
}

func TestEnsureBootstrapAdminValidatesPassword(t *testing.T) {
	if _, err := EnsureBootstrapAdmin(newMemoryRepo(), BootstrapAdminInput{Email: "a@mooc.test", Password: "corta"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("una clave corta debía fallar la validación, llegó %v", err)
	}
}
