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
		Role:            domain.RoleTeacher,
		Status:          domain.StatusActive,
		EmailVerifiedAt: &now,
	}
	if err := repo.CreateUser(u); err != nil {
		return nil, err
	}
	_ = audit(repo, &actorID, "admin.create_teacher", "user", &u.ID, map[string]any{"email": u.Email})
	return u, nil
}

func UpdateUser(repo domain.UserRepository, in UpdateUserInput) (*domain.User, error) {
	u, err := repo.FindUserByID(in.UserID)
	if err != nil {
		return nil, err
	}
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
	_ = audit(repo, &in.ActorID, "admin.update_user", "user", &u.ID, map[string]any{"role": u.Role, "status": u.Status})
	return u, nil
}

func DeleteUser(repo domain.UserRepository, actorID, userID string) error {
	status := domain.StatusDeleted
	_, err := UpdateUser(repo, UpdateUserInput{ActorID: actorID, UserID: userID, Status: &status})
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
