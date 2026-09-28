package usecase

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

const (
	SessionTTL       = 24 * time.Hour
	ResetTokenTTL    = 30 * time.Minute
	minPasswordRunes = 8
)

type RegisterInput struct {
	Email    string
	Password string
	FullName string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthResult struct {
	User      *domain.User    `json:"user"`
	Session   *domain.Session `json:"session,omitempty"`
	Token     string          `json:"token,omitempty"`
	ExpiresAt time.Time       `json:"expires_at,omitempty"`
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func validateUserInput(email, password, fullName string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.Join(domain.ErrValidation, errors.New("email invalido"))
	}
	if len([]rune(password)) < minPasswordRunes {
		return errors.Join(domain.ErrValidation, errors.New("la clave debe tener al menos 8 caracteres"))
	}
	if strings.TrimSpace(fullName) == "" {
		return errors.Join(domain.ErrValidation, errors.New("el nombre es obligatorio"))
	}
	return nil
}

func Register(repo domain.UserRepository, mailer domain.Mailer, in RegisterInput) (*domain.User, string, error) {
	in.Email = NormalizeEmail(in.Email)
	if err := validateUserInput(in.Email, in.Password, in.FullName); err != nil {
		return nil, "", err
	}
	if _, err := repo.FindUserByEmail(in.Email); err == nil {
		return nil, "", domain.ErrEmailAlreadyTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	token, err := NewToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now()
	tokenHash := HashToken(token)
	u := &domain.User{
		Email:                 in.Email,
		PasswordHash:          string(hash),
		FullName:              strings.TrimSpace(in.FullName),
		Role:                  domain.RoleStudent,
		Status:                domain.StatusPendingVerification,
		VerificationTokenHash: &tokenHash,
		VerificationSentAt:    &now,
	}
	if err := repo.CreateUser(u); err != nil {
		return nil, "", err
	}
	_ = repo.CreateAuditLog(&domain.AuditLog{Action: "auth.register", EntityType: "user", EntityID: &u.ID, Metadata: "{}"})
	_ = mailer.Send(u.Email, "Verifica tu correo", fmt.Sprintf("Token de verificacion: %s", token))
	return u, token, nil
}

func BootstrapAdmin(repo domain.UserRepository, in RegisterInput) (*domain.User, error) {
	count, err := repo.CountActiveAdmins()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, domain.ErrAdminAlreadyExists
	}
	in.Email = NormalizeEmail(in.Email)
	if err := validateUserInput(in.Email, in.Password, in.FullName); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	u := &domain.User{
		Email:           in.Email,
		PasswordHash:    string(hash),
		FullName:        strings.TrimSpace(in.FullName),
		Role:            domain.RoleAdmin,
		Status:          domain.StatusActive,
		EmailVerifiedAt: &now,
	}
	if err := repo.CreateUser(u); err != nil {
		return nil, err
	}
	_ = repo.CreateAuditLog(&domain.AuditLog{Action: "admin.bootstrap", EntityType: "user", EntityID: &u.ID, Metadata: "{}"})
	return u, nil
}

func VerifyEmail(repo domain.UserRepository, token string) (*domain.User, error) {
	u, err := repo.FindUserByVerificationTokenHash(HashToken(token))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}
	now := time.Now()
	if err := repo.MarkEmailVerified(u.ID, now); err != nil {
		return nil, err
	}
	u.Status = domain.StatusActive
	u.EmailVerifiedAt = &now
	_ = repo.CreateAuditLog(&domain.AuditLog{Action: "auth.verify_email", EntityType: "user", EntityID: &u.ID, Metadata: "{}"})
	return u, nil
}

func Login(repo domain.UserRepository, store domain.SessionStore, in LoginInput) (*AuthResult, error) {
	u, err := repo.FindUserByEmail(NormalizeEmail(in.Email))
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if u.Status != domain.StatusActive {
		return nil, domain.ErrInactiveUser
	}
	token, err := NewToken()
	if err != nil {
		return nil, err
	}
	tokenHash := HashToken(token)
	expiresAt := time.Now().Add(SessionTTL)
	s := &domain.Session{UserID: u.ID, TokenHash: tokenHash, ExpiresAt: expiresAt}
	if err := repo.CreateSession(s); err != nil {
		return nil, err
	}
	if err := store.Save(tokenHash, *u, expiresAt); err != nil {
		return nil, err
	}
	_ = repo.CreateAuditLog(&domain.AuditLog{ActorID: &u.ID, Action: "auth.login", EntityType: "session", EntityID: &s.ID, Metadata: "{}"})
	return &AuthResult{User: u, Session: s, Token: token, ExpiresAt: expiresAt}, nil
}

func Authenticate(repo domain.UserRepository, store domain.SessionStore, token string) (*domain.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, domain.ErrUnauthorized
	}
	tokenHash := HashToken(token)
	if u, err := store.Get(tokenHash); err == nil {
		fresh, err := repo.FindUserByID(u.ID)
		if err != nil {
			return nil, err
		}
		if fresh.Status != domain.StatusActive {
			return nil, domain.ErrInactiveUser
		}
		return fresh, nil
	}
	s, err := repo.FindSessionByTokenHash(tokenHash)
	if err != nil || s.RevokedAt != nil || time.Now().After(s.ExpiresAt) {
		return nil, domain.ErrUnauthorized
	}
	u, err := repo.FindUserByID(s.UserID)
	if err != nil {
		return nil, err
	}
	if u.Status != domain.StatusActive {
		return nil, domain.ErrInactiveUser
	}
	_ = store.Save(tokenHash, *u, s.ExpiresAt)
	return u, nil
}

func Logout(repo domain.UserRepository, store domain.SessionStore, token string) error {
	tokenHash := HashToken(token)
	now := time.Now()
	_ = store.Delete(tokenHash)
	session, err := repo.FindSessionByTokenHash(tokenHash)
	if err != nil {
		return err
	}
	if err := repo.RevokeSessionByTokenHash(tokenHash, now); err != nil {
		return err
	}
	_ = repo.CreateAuditLog(&domain.AuditLog{ActorID: &session.UserID, Action: "auth.logout", EntityType: "session", EntityID: &session.ID, Metadata: "{}"})
	return nil
}

func ForgotPassword(repo domain.UserRepository, mailer domain.Mailer, email string) error {
	u, err := repo.FindUserByEmail(NormalizeEmail(email))
	if err != nil {
		return nil
	}
	token, err := NewToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(ResetTokenTTL)
	if err := repo.SetResetToken(u.ID, HashToken(token), expiresAt); err != nil {
		return err
	}
	_ = mailer.Send(u.Email, "Recupera tu clave", fmt.Sprintf("Token de recuperacion: %s", token))
	_ = repo.CreateAuditLog(&domain.AuditLog{ActorID: &u.ID, Action: "auth.password_reset_requested", EntityType: "user", EntityID: &u.ID, Metadata: "{}"})
	return nil
}

func ResetPassword(repo domain.UserRepository, token, newPassword string) error {
	if len([]rune(newPassword)) < minPasswordRunes {
		return errors.Join(domain.ErrValidation, errors.New("la clave debe tener al menos 8 caracteres"))
	}
	u, err := repo.FindUserByResetTokenHash(HashToken(token))
	if err != nil || u.ResetTokenExpiresAt == nil || time.Now().After(*u.ResetTokenExpiresAt) {
		return domain.ErrInvalidToken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := repo.UpdatePassword(u.ID, string(hash)); err != nil {
		return err
	}
	_ = repo.ClearResetToken(u.ID)
	_ = repo.CreateAuditLog(&domain.AuditLog{ActorID: &u.ID, Action: "auth.password_reset_completed", EntityType: "user", EntityID: &u.ID, Metadata: "{}"})
	return nil
}
