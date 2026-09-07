package usecase

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

func TestRegisterCreatesPendingStudentAndVerificationToken(t *testing.T) {
	repo := newMemoryRepo()
	mailer := &memoryMailer{}

	user, token, err := Register(repo, mailer, RegisterInput{
		Email: "ANA@Example.COM", Password: "claveSegura123", FullName: "Ana Perez",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.Role != domain.RoleStudent || user.Status != domain.StatusPendingVerification {
		t.Fatalf("user role/status = %s/%s", user.Role, user.Status)
	}
	if user.Email != "ana@example.com" {
		t.Fatalf("email was not normalized: %s", user.Email)
	}
	if token == "" || user.VerificationTokenHash == nil {
		t.Fatalf("expected verification token")
	}
	if len(mailer.messages) != 1 || !strings.Contains(mailer.messages[0], token) {
		t.Fatalf("expected verification email with token")
	}
}

func TestVerifyEmailThenLoginCreatesSession(t *testing.T) {
	repo := newMemoryRepo()
	store := newMemoryStore()
	mailer := &memoryMailer{}

	user, token, err := Register(repo, mailer, RegisterInput{
		Email: "ana@example.com", Password: "claveSegura123", FullName: "Ana Perez",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	verified, err := VerifyEmail(repo, token)
	if err != nil {
		t.Fatalf("VerifyEmail() error = %v", err)
	}
	if verified.Status != domain.StatusActive {
		t.Fatalf("status = %s", verified.Status)
	}

	result, err := Login(repo, store, LoginInput{Email: user.Email, Password: "claveSegura123"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Token == "" || result.Session == nil {
		t.Fatalf("expected token and session")
	}
	authenticated, err := Authenticate(repo, store, result.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("authenticated user = %s, want %s", authenticated.ID, user.ID)
	}
}

func TestBootstrapAdminOnlyWhenNoActiveAdmin(t *testing.T) {
	repo := newMemoryRepo()
	first, err := BootstrapAdmin(repo, RegisterInput{Email: "admin@example.com", Password: "claveSegura123", FullName: "Admin"})
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	if first.Role != domain.RoleAdmin || first.Status != domain.StatusActive {
		t.Fatalf("admin role/status = %s/%s", first.Role, first.Status)
	}
	_, err = BootstrapAdmin(repo, RegisterInput{Email: "other@example.com", Password: "claveSegura123", FullName: "Other"})
	if !errors.Is(err, domain.ErrAdminAlreadyExists) {
		t.Fatalf("second BootstrapAdmin() error = %v, want ErrAdminAlreadyExists", err)
	}
}

func TestProtectsLastActiveAdmin(t *testing.T) {
	repo := newMemoryRepo()
	admin, err := BootstrapAdmin(repo, RegisterInput{Email: "admin@example.com", Password: "claveSegura123", FullName: "Admin"})
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	status := domain.StatusSuspended
	_, err = UpdateUser(repo, UpdateUserInput{ActorID: "another-admin", UserID: admin.ID, Status: &status})
	if !errors.Is(err, domain.ErrLastActiveAdmin) {
		t.Fatalf("UpdateUser() error = %v, want ErrLastActiveAdmin", err)
	}
}

type memoryRepo struct {
	users    map[string]*domain.User
	sessions map[string]*domain.Session
	audit    []domain.AuditLog
	nextID   int
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{users: map[string]*domain.User{}, sessions: map[string]*domain.Session{}}
}

func (r *memoryRepo) id() string {
	r.nextID++
	return strings.Repeat("0", 35) + string(rune('0'+r.nextID))
}

func (r *memoryRepo) CreateUser(u *domain.User) error {
	u.ID = r.id()
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *memoryRepo) FindUserByID(id string) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok || u.Status == domain.StatusDeleted {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *memoryRepo) FindUserByEmail(email string) (*domain.User, error) {
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) && u.Status != domain.StatusDeleted {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memoryRepo) FindUserByVerificationTokenHash(hash string) (*domain.User, error) {
	for _, u := range r.users {
		if u.VerificationTokenHash != nil && *u.VerificationTokenHash == hash {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memoryRepo) FindUserByResetTokenHash(hash string) (*domain.User, error) {
	for _, u := range r.users {
		if u.ResetTokenHash != nil && *u.ResetTokenHash == hash {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memoryRepo) ListUsers() ([]domain.User, error) {
	out := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, *u)
	}
	return out, nil
}

func (r *memoryRepo) UpdateUser(u *domain.User) error {
	current, ok := r.users[u.ID]
	if !ok {
		return domain.ErrNotFound
	}
	*current = *u
	return nil
}

func (r *memoryRepo) UpdatePassword(userID, passwordHash string) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.PasswordHash = passwordHash
	return nil
}

func (r *memoryRepo) SetVerificationToken(userID, tokenHash string, sentAt time.Time) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.VerificationTokenHash = &tokenHash
	u.VerificationSentAt = &sentAt
	return nil
}

func (r *memoryRepo) MarkEmailVerified(userID string, verifiedAt time.Time) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.Status = domain.StatusActive
	u.EmailVerifiedAt = &verifiedAt
	u.VerificationTokenHash = nil
	return nil
}

func (r *memoryRepo) SetResetToken(userID, tokenHash string, expiresAt time.Time) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.ResetTokenHash = &tokenHash
	u.ResetTokenExpiresAt = &expiresAt
	return nil
}

func (r *memoryRepo) ClearResetToken(userID string) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.ResetTokenHash = nil
	u.ResetTokenExpiresAt = nil
	return nil
}

func (r *memoryRepo) CountActiveAdmins() (int, error) {
	count := 0
	for _, u := range r.users {
		if u.Role == domain.RoleAdmin && u.Status == domain.StatusActive {
			count++
		}
	}
	return count, nil
}

func (r *memoryRepo) CreateSession(s *domain.Session) error {
	s.ID = r.id()
	s.CreatedAt = time.Now()
	s.UpdatedAt = s.CreatedAt
	cp := *s
	r.sessions[s.TokenHash] = &cp
	return nil
}

func (r *memoryRepo) FindSessionByTokenHash(tokenHash string) (*domain.Session, error) {
	s, ok := r.sessions[tokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *memoryRepo) FindSessionByID(id string) (*domain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			cp := *s
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memoryRepo) ListSessionsByUser(userID string) ([]domain.Session, error) {
	var out []domain.Session
	for _, s := range r.sessions {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (r *memoryRepo) RevokeSession(sessionID string, revokedAt time.Time) error {
	for _, s := range r.sessions {
		if s.ID == sessionID {
			s.RevokedAt = &revokedAt
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *memoryRepo) RevokeSessionByTokenHash(tokenHash string, revokedAt time.Time) error {
	s, ok := r.sessions[tokenHash]
	if !ok {
		return domain.ErrNotFound
	}
	s.RevokedAt = &revokedAt
	return nil
}

func (r *memoryRepo) CreateAuditLog(a *domain.AuditLog) error {
	r.audit = append(r.audit, *a)
	return nil
}

func (r *memoryRepo) ListAuditLog() ([]domain.AuditLog, error) {
	return r.audit, nil
}

type memoryStore struct {
	users map[string]domain.User
}

func newMemoryStore() *memoryStore {
	return &memoryStore{users: map[string]domain.User{}}
}

func (s *memoryStore) Save(tokenHash string, user domain.User, _ time.Time) error {
	s.users[tokenHash] = user
	return nil
}

func (s *memoryStore) Get(tokenHash string) (*domain.User, error) {
	u, ok := s.users[tokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (s *memoryStore) Delete(tokenHash string) error {
	delete(s.users, tokenHash)
	return nil
}

type memoryMailer struct {
	messages []string
}

func (m *memoryMailer) Send(to, subject, body string) error {
	m.messages = append(m.messages, to+" "+subject+" "+body)
	return nil
}
