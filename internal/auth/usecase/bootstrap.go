package usecase

import (
	"errors"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

// BootstrapAdminInput son los datos de la primera cuenta de administrador.
type BootstrapAdminInput struct {
	Email    string
	FullName string
	Password string
}

// Resultados posibles de EnsureBootstrapAdmin (para el registro de arranque).
const (
	BootstrapDisabled     = "desactivado"
	BootstrapAdminExists  = "ya hay un administrador activo"
	BootstrapAdminCreated = "administrador creado"
)

// ErrBootstrapEmailTaken indica que el correo del administrador inicial ya
// pertenece a una cuenta que no es administradora. No se promueve: alguien
// pudo registrarse con ese correo antes del primer arranque.
var ErrBootstrapEmailTaken = errors.New("el correo del administrador inicial ya pertenece a otra cuenta")

// EnsureBootstrapAdmin crea la primera cuenta de administrador cuando la
// base no tiene ninguno activo (Entrega 4).
//
// En la E3 la cuenta admin se creaba desde la VM de servicios, que llegaba a
// Cloud SQL por la VPC. En Cloud Run no hay VM: el backend lee el correo de
// BOOTSTRAP_ADMIN_EMAIL y la clave del secreto mooc-admin-password y, al
// arrancar, crea la cuenta solo si no existe ningún administrador activo.
// Una vez creada, esto no vuelve a hacer nada: cambiar el secreto no cambia
// la clave ni crea otra cuenta. Así tampoco queda abierto el endpoint
// público POST /auth/bootstrap-admin, que solo funciona sin administradores.
func EnsureBootstrapAdmin(repo domain.UserRepository, in BootstrapAdminInput) (string, error) {
	in.Email = NormalizeEmail(in.Email)
	if in.Email == "" || in.Password == "" {
		return BootstrapDisabled, nil
	}
	if in.FullName == "" {
		in.FullName = "Administrador MOOC"
	}

	// Antes de crear, no promover ni chocar con una cuenta que ya use el
	// correo (alguien pudo registrarse con él antes del primer arranque).
	if admins, err := repo.CountActiveAdmins(); err != nil {
		return "", err
	} else if admins > 0 {
		return BootstrapAdminExists, nil
	}
	if _, err := repo.FindUserByEmail(in.Email); err == nil {
		return "", ErrBootstrapEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}

	// La creación es la misma de POST /auth/bootstrap-admin.
	if _, err := BootstrapAdmin(repo, RegisterInput{Email: in.Email, Password: in.Password, FullName: in.FullName}); err != nil {
		if errors.Is(err, domain.ErrAdminAlreadyExists) {
			return BootstrapAdminExists, nil
		}
		return "", err
	}
	return BootstrapAdminCreated, nil
}
