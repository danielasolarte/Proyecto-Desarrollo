package domain

import (
	"errors"
	"time"
)

type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

type Status string

const (
	StatusPendingVerification Status = "pending_verification"
	StatusActive              Status = "active"
	StatusSuspended           Status = "suspended"
	StatusDeleted             Status = "deleted"
)

type User struct {
	ID                    string     `json:"id"`
	Email                 string     `json:"email"`
	PasswordHash          string     `json:"-"`
	FullName              string     `json:"full_name"`
	Role                  Role       `json:"role"`
	Status                Status     `json:"status"`
	EmailVerifiedAt       *time.Time `json:"email_verified_at,omitempty"`
	VerificationSentAt    *time.Time `json:"verification_sent_at,omitempty"`
	ResetTokenExpiresAt   *time.Time `json:"reset_token_expires_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	VerificationTokenHash *string    `json:"-"`
	ResetTokenHash        *string    `json:"-"`
}

type Session struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"-"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type AuditLog struct {
	ID         string    `json:"id"`
	ActorID    *string   `json:"actor_id,omitempty"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	EntityID   *string   `json:"entity_id,omitempty"`
	Metadata   string    `json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
}

var (
	ErrNotFound            = errors.New("recurso no encontrado")
	ErrValidation          = errors.New("error de validacion")
	ErrEmailAlreadyTaken   = errors.New("email ya registrado")
	ErrInvalidCredentials  = errors.New("credenciales invalidas")
	ErrUnauthorized        = errors.New("no autenticado")
	ErrForbidden           = errors.New("no autorizado")
	ErrInactiveUser        = errors.New("usuario inactivo")
	ErrLastActiveAdmin     = errors.New("no se puede modificar el ultimo administrador activo")
	ErrSessionRevoked      = errors.New("sesion revocada")
	ErrInvalidToken        = errors.New("token invalido")
	ErrAdminAlreadyExists  = errors.New("ya existe un administrador activo")
	ErrCannotSelfDowngrade = errors.New("no puedes degradar o suspender tu propia cuenta admin")
)

type UserRepository interface {
	CreateUser(u *User) error
	FindUserByID(id string) (*User, error)
	FindUserByEmail(email string) (*User, error)
	FindUserByVerificationTokenHash(hash string) (*User, error)
	FindUserByResetTokenHash(hash string) (*User, error)
	ListUsers() ([]User, error)
	UpdateUser(u *User) error
	UpdatePassword(userID, passwordHash string) error
	SetVerificationToken(userID, tokenHash string, sentAt time.Time) error
	MarkEmailVerified(userID string, verifiedAt time.Time) error
	SetResetToken(userID, tokenHash string, expiresAt time.Time) error
	ClearResetToken(userID string) error
	CountActiveAdmins() (int, error)

	CreateSession(s *Session) error
	FindSessionByID(id string) (*Session, error)
	FindSessionByTokenHash(tokenHash string) (*Session, error)
	ListSessionsByUser(userID string) ([]Session, error)
	RevokeSession(sessionID string, revokedAt time.Time) error
	RevokeSessionsByUser(userID string, revokedAt time.Time) error
	RevokeSessionByTokenHash(tokenHash string, revokedAt time.Time) error

	CreateAuditLog(a *AuditLog) error
	ListAuditLog() ([]AuditLog, error)
}

type SessionStore interface {
	Save(tokenHash string, user User, expiresAt time.Time) error
	Get(tokenHash string) (*User, error)
	Delete(tokenHash string) error
}

type Mailer interface {
	Send(to, subject, body string) error
}
