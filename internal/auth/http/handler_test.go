package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/auth/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

func TestRegisterEndpointCreatesPendingStudent(t *testing.T) {
	repo := newHTTPRepo()
	mailer := &fakeMailer{}
	e := echo.New()
	RegisterRoutes(e, NewHandler(repo, newHTTPStore(), mailer))

	body := `{"email":"ana@example.com","password":"claveSegura123","full_name":"Ana Perez"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	user, err := repo.FindUserByEmail("ana@example.com")
	if err != nil {
		t.Fatalf("expected user to be persisted: %v", err)
	}
	if user.Role != domain.RoleStudent || user.Status != domain.StatusPendingVerification {
		t.Fatalf("role/status = %s/%s", user.Role, user.Status)
	}
	if len(mailer.messages) != 1 {
		t.Fatalf("expected one verification email, got %d", len(mailer.messages))
	}
}

func TestProtectedRouteRequiresTokenAndRole(t *testing.T) {
	repo := newHTTPRepo()
	store := newHTTPStore()
	admin := repo.mustUser(domain.RoleAdmin)
	student := repo.mustUser(domain.RoleStudent)
	adminToken := "admin-token"
	studentToken := "student-token"
	store.Save(usecase.HashToken(adminToken), *admin, time.Now().Add(time.Hour))
	store.Save(usecase.HashToken(studentToken), *student, time.Now().Add(time.Hour))

	e := echo.New()
	e.Use(AuthMiddleware(repo, store))
	e.GET("/admin-only", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, authctx.RequireRole(authctx.RoleAdmin))

	assertStatus := func(name, token string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s: status = %d, want %d, body = %s", name, rec.Code, want, rec.Body.String())
		}
	}

	assertStatus("missing token", "", http.StatusUnauthorized)
	assertStatus("student token", studentToken, http.StatusForbidden)
	assertStatus("admin token", adminToken, http.StatusOK)
}

type httpRepo struct {
	users        map[string]*domain.User
	sessions     map[string]*domain.Session
	next         int
	auditEntries []domain.AuditLog
}

func newHTTPRepo() *httpRepo {
	return &httpRepo{users: map[string]*domain.User{}, sessions: map[string]*domain.Session{}}
}

func (r *httpRepo) nextID() string {
	r.next++
	return "00000000-0000-0000-0000-00000000000" + string(rune('0'+r.next))
}

func (r *httpRepo) mustUser(role domain.Role) *domain.User {
	now := time.Now()
	u := &domain.User{
		ID:              r.nextID(),
		Email:           string(role) + "@example.com",
		PasswordHash:    "unused",
		FullName:        string(role),
		Role:            role,
		Status:          domain.StatusActive,
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	r.users[u.ID] = u
	return u
}

func (r *httpRepo) CreateUser(u *domain.User) error {
	now := time.Now()
	u.ID = r.nextID()
	u.CreatedAt = now
	u.UpdatedAt = now
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *httpRepo) FindUserByID(id string) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok || u.Status == domain.StatusDeleted {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *httpRepo) FindUserByEmail(email string) (*domain.User, error) {
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) && u.Status != domain.StatusDeleted {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *httpRepo) FindUserByVerificationTokenHash(hash string) (*domain.User, error) {
	for _, u := range r.users {
		if u.VerificationTokenHash != nil && *u.VerificationTokenHash == hash {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *httpRepo) FindUserByResetTokenHash(hash string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}

func (r *httpRepo) ListUsers() ([]domain.User, error) {
	out := []domain.User{}
	for _, u := range r.users {
		out = append(out, *u)
	}
	return out, nil
}

func (r *httpRepo) UpdateUser(u *domain.User) error {
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *httpRepo) UpdatePassword(userID, passwordHash string) error { return nil }

func (r *httpRepo) SetVerificationToken(userID, tokenHash string, sentAt time.Time) error {
	return nil
}

func (r *httpRepo) MarkEmailVerified(userID string, verifiedAt time.Time) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.Status = domain.StatusActive
	u.EmailVerifiedAt = &verifiedAt
	u.VerificationTokenHash = nil
	return nil
}

func (r *httpRepo) SetResetToken(userID, tokenHash string, expiresAt time.Time) error {
	return nil
}

func (r *httpRepo) ClearResetToken(userID string) error { return nil }

func (r *httpRepo) CountActiveAdmins() (int, error) {
	count := 0
	for _, u := range r.users {
		if u.Role == domain.RoleAdmin && u.Status == domain.StatusActive {
			count++
		}
	}
	return count, nil
}

func (r *httpRepo) CreateSession(s *domain.Session) error {
	s.ID = r.nextID()
	cp := *s
	r.sessions[s.TokenHash] = &cp
	return nil
}

func (r *httpRepo) FindSessionByID(id string) (*domain.Session, error) {
	for _, s := range r.sessions {
		if s.ID == id {
			cp := *s
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *httpRepo) FindSessionByTokenHash(tokenHash string) (*domain.Session, error) {
	s, ok := r.sessions[tokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *httpRepo) ListSessionsByUser(userID string) ([]domain.Session, error) {
	return nil, nil
}

func (r *httpRepo) RevokeSession(sessionID string, revokedAt time.Time) error {
	return nil
}

func (r *httpRepo) RevokeSessionsByUser(userID string, revokedAt time.Time) error {
	return nil
}

func (r *httpRepo) RevokeSessionByTokenHash(tokenHash string, revokedAt time.Time) error {
	return nil
}

func (r *httpRepo) CreateAuditLog(a *domain.AuditLog) error {
	r.auditEntries = append(r.auditEntries, *a)
	return nil
}

func (r *httpRepo) ListAuditLog() ([]domain.AuditLog, error) {
	return r.auditEntries, nil
}

type httpStore struct {
	users map[string]domain.User
}

func newHTTPStore() *httpStore {
	return &httpStore{users: map[string]domain.User{}}
}

func (s *httpStore) Save(tokenHash string, user domain.User, expiresAt time.Time) error {
	s.users[tokenHash] = user
	return nil
}

func (s *httpStore) Get(tokenHash string) (*domain.User, error) {
	u, ok := s.users[tokenHash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (s *httpStore) Delete(tokenHash string) error {
	delete(s.users, tokenHash)
	return nil
}

type fakeMailer struct {
	messages []string
}

func (m *fakeMailer) Send(to, subject, body string) error {
	m.messages = append(m.messages, to+" "+subject+" "+body)
	return nil
}
