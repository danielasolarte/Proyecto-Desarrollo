package usecase

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

type UpdateUserInput struct {
	ActorID  string
	UserID   string
	FullName *string
	Role     *domain.Role
	Status   *domain.Status
}

func validRole(r domain.Role) bool {
	return r == domain.RoleStudent || r == domain.RoleTeacher || r == domain.RoleAdmin
}

func validStatus(s domain.Status) bool {
	return s == domain.StatusPendingVerification || s == domain.StatusActive || s == domain.StatusSuspended || s == domain.StatusDeleted
}

func CreateTeacher(repo domain.UserRepository, actorID string, in RegisterInput) (*domain.User, error) {
	return createStaffUser(repo, actorID, in, domain.RoleTeacher, "admin.create_teacher")
}

func CreateAdmin(repo domain.UserRepository, actorID string, in RegisterInput) (*domain.User, error) {
	return createStaffUser(repo, actorID, in, domain.RoleAdmin, "admin.create_admin")
}

func createStaffUser(repo domain.UserRepository, actorID string, in RegisterInput, role domain.Role, action string) (*domain.User, error) {
	in.Email = NormalizeEmail(in.Email)
	if err := validateUserInput(in.Email, in.Password, in.FullName); err != nil {
		return nil, err
	}
	if _, err := repo.FindUserByEmail(in.Email); err == nil {
		return nil, domain.ErrEmailAlreadyTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
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
		Role:            role,
		Status:          domain.StatusActive,
		EmailVerifiedAt: &now,
	}
	if err := repo.CreateUser(u); err != nil {
		return nil, err
	}
	_ = audit(repo, &actorID, action, "user", &u.ID, map[string]any{"email": u.Email, "role": u.Role})
	return u, nil
}

func UpdateUser(repo domain.UserRepository, store domain.SessionStore, in UpdateUserInput) (*domain.User, error) {
	u, err := repo.FindUserByID(in.UserID)
	if err != nil {
		return nil, err
	}
	previousStatus := u.Status
	if in.FullName != nil && strings.TrimSpace(*in.FullName) != "" {
		u.FullName = strings.TrimSpace(*in.FullName)
	}
	if in.Role != nil {
		if !validRole(*in.Role) {
			return nil, errors.Join(domain.ErrValidation, errors.New("rol invalido"))
		}
		u.Role = *in.Role
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			return nil, errors.Join(domain.ErrValidation, errors.New("estado invalido"))
		}
		u.Status = *in.Status
	}
	if u.ID == in.ActorID && (u.Role != domain.RoleAdmin || u.Status != domain.StatusActive) {
		return nil, domain.ErrCannotSelfDowngrade
	}
	if err := protectLastActiveAdmin(repo, u); err != nil {
		return nil, err
	}
	if err := repo.UpdateUser(u); err != nil {
		return nil, err
	}
	if previousStatus == domain.StatusActive && (u.Status == domain.StatusSuspended || u.Status == domain.StatusDeleted) {
		_ = revokeAllSessionsForUser(repo, store, u.ID)
	}
	_ = audit(repo, &in.ActorID, "admin.update_user", "user", &u.ID, map[string]any{"role": u.Role, "status": u.Status})
	return u, nil
}

func DeleteUser(repo domain.UserRepository, store domain.SessionStore, actorID, userID string) error {
	status := domain.StatusDeleted
	_, err := UpdateUser(repo, store, UpdateUserInput{ActorID: actorID, UserID: userID, Status: &status})
	return err
}

func RevokeSession(repo domain.UserRepository, store domain.SessionStore, actorID, sessionID string) error {
	session, err := repo.FindSessionByID(sessionID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := repo.RevokeSession(sessionID, now); err != nil {
		return err
	}
	_ = store.Delete(session.TokenHash)
	_ = audit(repo, &actorID, "admin.revoke_session", "session", &sessionID, nil)
	return nil
}

func RevokeAllSessions(repo domain.UserRepository, store domain.SessionStore, actorID, userID string) error {
	if err := revokeAllSessionsForUser(repo, store, userID); err != nil {
		return err
	}
	_ = audit(repo, &actorID, "admin.revoke_user_sessions", "user", &userID, nil)
	return nil
}

func revokeAllSessionsForUser(repo domain.UserRepository, store domain.SessionStore, userID string) error {
	sessions, err := repo.ListSessionsByUser(userID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := repo.RevokeSessionsByUser(userID, now); err != nil {
		return err
	}
	for _, session := range sessions {
		_ = store.Delete(session.TokenHash)
	}
	return nil
}

func protectLastActiveAdmin(repo domain.UserRepository, next *domain.User) error {
	if next.Role == domain.RoleAdmin && next.Status == domain.StatusActive {
		return nil
	}
	current, err := repo.FindUserByID(next.ID)
	if err != nil {
		return err
	}
	if current.Role != domain.RoleAdmin || current.Status != domain.StatusActive {
		return nil
	}
	count, err := repo.CountActiveAdmins()
	if err != nil {
		return err
	}
	if count <= 1 {
		return domain.ErrLastActiveAdmin
	}
	return nil
}

func audit(repo domain.UserRepository, actorID *string, action, entityType string, entityID *string, metadata map[string]any) error {
	raw := "{}"
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		raw = string(b)
	}
	return repo.CreateAuditLog(&domain.AuditLog{ActorID: actorID, Action: action, EntityType: entityType, EntityID: entityID, Metadata: raw})
}
