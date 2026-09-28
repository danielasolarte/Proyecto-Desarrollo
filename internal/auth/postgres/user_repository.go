package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func wrapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func strPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func timePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	v := nt.Time
	return &v
}

func nullableStr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullableTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func (r *UserRepository) CreateUser(u *domain.User) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO users (
			email, password_hash, full_name, role, status, email_verified_at,
			verification_token_hash, verification_sent_at, reset_token_hash, reset_token_expires_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, created_at, updated_at
	`, u.Email, u.PasswordHash, u.FullName, u.Role, u.Status, nullableTime(u.EmailVerifiedAt),
		nullableStr(u.VerificationTokenHash), nullableTime(u.VerificationSentAt),
		nullableStr(u.ResetTokenHash), nullableTime(u.ResetTokenExpiresAt)).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepository) FindUserByID(id string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(context.Background(), `
		SELECT id, email, password_hash, full_name, role, status, email_verified_at,
		       verification_token_hash, verification_sent_at, reset_token_hash, reset_token_expires_at,
		       created_at, updated_at
		FROM users WHERE id = $1 AND status <> 'deleted'
	`, id))
}

func (r *UserRepository) FindUserByEmail(email string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(context.Background(), `
		SELECT id, email, password_hash, full_name, role, status, email_verified_at,
		       verification_token_hash, verification_sent_at, reset_token_hash, reset_token_expires_at,
		       created_at, updated_at
		FROM users WHERE lower(email) = lower($1) AND status <> 'deleted'
	`, email))
}

func (r *UserRepository) FindUserByVerificationTokenHash(hash string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(context.Background(), `
		SELECT id, email, password_hash, full_name, role, status, email_verified_at,
		       verification_token_hash, verification_sent_at, reset_token_hash, reset_token_expires_at,
		       created_at, updated_at
		FROM users WHERE verification_token_hash = $1 AND status = 'pending_verification'
	`, hash))
}

func (r *UserRepository) FindUserByResetTokenHash(hash string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(context.Background(), `
		SELECT id, email, password_hash, full_name, role, status, email_verified_at,
		       verification_token_hash, verification_sent_at, reset_token_hash, reset_token_expires_at,
		       created_at, updated_at
		FROM users WHERE reset_token_hash = $1 AND status <> 'deleted'
	`, hash))
}

func (r *UserRepository) ListUsers() ([]domain.User, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, email, password_hash, full_name, role, status, email_verified_at,
		       verification_token_hash, verification_sent_at, reset_token_hash, reset_token_expires_at,
		       created_at, updated_at
		FROM users WHERE status <> 'deleted' ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (r *UserRepository) UpdateUser(u *domain.User) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE users
		SET full_name = $1, role = $2, status = $3, email_verified_at = $4, updated_at = now()
		WHERE id = $5
	`, u.FullName, u.Role, u.Status, nullableTime(u.EmailVerifiedAt), u.ID)
	return err
}

func (r *UserRepository) UpdatePassword(userID, passwordHash string) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2
	`, passwordHash, userID)
	return err
}

func (r *UserRepository) SetVerificationToken(userID, tokenHash string, sentAt time.Time) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE users SET verification_token_hash = $1, verification_sent_at = $2, updated_at = now()
		WHERE id = $3
	`, tokenHash, sentAt, userID)
	return err
}

func (r *UserRepository) MarkEmailVerified(userID string, verifiedAt time.Time) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE users
		SET status = 'active', email_verified_at = $1, verification_token_hash = NULL, updated_at = now()
		WHERE id = $2
	`, verifiedAt, userID)
	return err
}

func (r *UserRepository) SetResetToken(userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE users SET reset_token_hash = $1, reset_token_expires_at = $2, updated_at = now()
		WHERE id = $3
	`, tokenHash, expiresAt, userID)
	return err
}

func (r *UserRepository) ClearResetToken(userID string) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE users SET reset_token_hash = NULL, reset_token_expires_at = NULL, updated_at = now()
		WHERE id = $1
	`, userID)
	return err
}

func (r *UserRepository) CountActiveAdmins() (int, error) {
	var count int
	err := r.db.QueryRow(context.Background(), `
		SELECT count(*) FROM users WHERE role = 'admin' AND status = 'active'
	`).Scan(&count)
	return count, err
}

func (r *UserRepository) CreateSession(s *domain.Session) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`, s.UserID, s.TokenHash, s.ExpiresAt).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *UserRepository) FindSessionByTokenHash(tokenHash string) (*domain.Session, error) {
	return scanSession(r.db.QueryRow(context.Background(), `
		SELECT id, user_id, token_hash, revoked_at, expires_at, created_at, updated_at
		FROM sessions WHERE token_hash = $1
	`, tokenHash))
}

func (r *UserRepository) FindSessionByID(id string) (*domain.Session, error) {
	return scanSession(r.db.QueryRow(context.Background(), `
		SELECT id, user_id, token_hash, revoked_at, expires_at, created_at, updated_at
		FROM sessions WHERE id = $1
	`, id))
}

func (r *UserRepository) ListSessionsByUser(userID string) ([]domain.Session, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, user_id, token_hash, revoked_at, expires_at, created_at, updated_at
		FROM sessions WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *UserRepository) RevokeSession(sessionID string, revokedAt time.Time) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE sessions SET revoked_at = $1, updated_at = now() WHERE id = $2
	`, revokedAt, sessionID)
	return err
}

func (r *UserRepository) RevokeSessionsByUser(userID string, revokedAt time.Time) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE sessions SET revoked_at = $1, updated_at = now()
		WHERE user_id = $2 AND revoked_at IS NULL
	`, revokedAt, userID)
	return err
}

func (r *UserRepository) RevokeSessionByTokenHash(tokenHash string, revokedAt time.Time) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE sessions SET revoked_at = $1, updated_at = now() WHERE token_hash = $2
	`, revokedAt, tokenHash)
	return err
}

func (r *UserRepository) CreateAuditLog(a *domain.AuditLog) error {
	return r.db.QueryRow(context.Background(), `
		INSERT INTO audit_log (actor_id, action, entity_type, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id, created_at
	`, nullableStr(a.ActorID), a.Action, a.EntityType, nullableStr(a.EntityID), a.Metadata).
		Scan(&a.ID, &a.CreatedAt)
}

func (r *UserRepository) ListAuditLog() ([]domain.AuditLog, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, actor_id, action, entity_type, entity_id, metadata::text, created_at
		FROM audit_log ORDER BY created_at DESC LIMIT 200
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		var actorID, entityID sql.NullString
		if err := rows.Scan(&a.ID, &actorID, &a.Action, &a.EntityType, &entityID, &a.Metadata, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.ActorID = strPtr(actorID)
		a.EntityID = strPtr(entityID)
		out = append(out, a)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	var verifiedAt, verificationSentAt, resetExpiresAt sql.NullTime
	var verificationHash, resetHash sql.NullString
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.Status,
		&verifiedAt, &verificationHash, &verificationSentAt, &resetHash, &resetExpiresAt,
		&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	u.EmailVerifiedAt = timePtr(verifiedAt)
	u.VerificationTokenHash = strPtr(verificationHash)
	u.VerificationSentAt = timePtr(verificationSentAt)
	u.ResetTokenHash = strPtr(resetHash)
	u.ResetTokenExpiresAt = timePtr(resetExpiresAt)
	return &u, nil
}

func scanSession(row rowScanner) (*domain.Session, error) {
	var s domain.Session
	var revokedAt sql.NullTime
	err := row.Scan(&s.ID, &s.UserID, &s.TokenHash, &revokedAt, &s.ExpiresAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, wrapErr(err)
	}
	s.RevokedAt = timePtr(revokedAt)
	return &s, nil
}
