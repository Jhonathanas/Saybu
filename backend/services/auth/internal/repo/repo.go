// Package repo mengakses database saybu_auth dengan pgx.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
	"saybu/services/auth/internal/domain"
)

// Repo adalah implementasi akses data auth.
type Repo struct {
	pool *pgxpool.Pool
}

// New membuat Repo.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const userColumns = `id, name, phone, email, password_hash, role, created_at`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	var email *string
	var role string
	if err := row.Scan(&u.ID, &u.Name, &u.Phone, &email, &u.PasswordHash, &role, &u.CreatedAt); err != nil {
		return domain.User{}, err
	}
	u.Email = email
	u.Role = domain.Role(role)
	return u, nil
}

// CreateUser menyimpan user baru.
func (r *Repo) CreateUser(ctx context.Context, u domain.User) (domain.User, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, name, phone, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, u.ID, u.Name, u.Phone, u.Email, u.PasswordHash, string(u.Role))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, domain.ErrUserExists
		}
		return domain.User{}, fmt.Errorf("repo: create user: %w", err)
	}
	return r.GetUserByID(ctx, u.ID)
}

// GetUserByID mengambil user berdasarkan id.
func (r *Repo) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("repo: get user by id: %w", err)
	}
	return u, nil
}

// GetUserByIdentifier mengambil user berdasarkan nomor HP atau email.
func (r *Repo) GetUserByIdentifier(ctx context.Context, identifier string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE phone = $1 OR email = $1::citext`, identifier))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("repo: get user by identifier: %w", err)
	}
	return u, nil
}

// ListUsers mengambil daftar user dengan pencarian opsional.
func (r *Repo) ListUsers(ctx context.Context, query string, limit, offset int) ([]domain.User, int64, error) {
	q := strings.TrimSpace(query)

	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM users
		WHERE $1 = ''
		   OR name ILIKE '%' || $1 || '%'
		   OR phone ILIKE '%' || $1 || '%'
		   OR COALESCE(email::text, '') ILIKE '%' || $1 || '%'
	`, q).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repo: count users: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE $1 = ''
		   OR name ILIKE '%' || $1 || '%'
		   OR phone ILIKE '%' || $1 || '%'
		   OR COALESCE(email::text, '') ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repo: list users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repo: scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repo: rows users: %w", err)
	}
	return users, total, nil
}

// CreateRefreshToken menyimpan sesi refresh token.
func (r *Repo) CreateRefreshToken(ctx context.Context, t domain.RefreshToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.UserAgent, t.IP)
	if err != nil {
		return fmt.Errorf("repo: create refresh token: %w", err)
	}
	return nil
}

// RotateRefreshToken mencabut token lama dan membuat token baru dalam satu transaksi.
// Bila token lama sudah pernah dicabut (reuse), semua sesi user dicabut.
func (r *Repo) RotateRefreshToken(ctx context.Context, oldHash string, newToken domain.RefreshToken, now time.Time) (domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("repo: begin rotate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id, userID string
	var expiresAt time.Time
	var revokedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, oldHash).Scan(&id, &userID, &expiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrInvalidToken
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("repo: get refresh token: %w", err)
	}

	if revokedAt != nil {
		if _, err := tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`,
			userID, now); err != nil {
			return domain.User{}, fmt.Errorf("repo: revoke all sessions: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.User{}, fmt.Errorf("repo: commit revoke all: %w", err)
		}
		return domain.User{}, domain.ErrTokenReused
	}

	if !expiresAt.After(now) {
		return domain.User{}, domain.ErrInvalidToken
	}

	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE id = $1`, id, now); err != nil {
		return domain.User{}, fmt.Errorf("repo: revoke old token: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, newToken.ID, userID, newToken.TokenHash, newToken.ExpiresAt, newToken.UserAgent, newToken.IP); err != nil {
		return domain.User{}, fmt.Errorf("repo: insert rotated token: %w", err)
	}

	u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, userID))
	if err != nil {
		return domain.User{}, fmt.Errorf("repo: get rotated user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("repo: commit rotate: %w", err)
	}
	return u, nil
}

// RevokeRefreshTokenByHash mencabut satu token (logout).
func (r *Repo) RevokeRefreshTokenByHash(ctx context.Context, hash string, now time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`,
		hash, now)
	if err != nil {
		return fmt.Errorf("repo: revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllRefreshTokens mencabut semua sesi user.
func (r *Repo) RevokeAllRefreshTokens(ctx context.Context, userID string, now time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`,
		userID, now)
	if err != nil {
		return fmt.Errorf("repo: revoke all refresh tokens: %w", err)
	}
	return nil
}

// InsertAudit menyimpan satu entri audit. request_id yang sama tidak ditulis ulang
// (idempotensi saat gateway retry); request_id kosong selalu ditulis.
func (r *Repo) InsertAudit(ctx context.Context, a domain.AuditEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, actor_id, actor_role, action, path, status, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (request_id) WHERE request_id <> '' DO NOTHING
	`, ids.New(), a.ActorID, a.ActorRole, a.Action, a.Path, a.Status, a.RequestID)
	if err != nil {
		return fmt.Errorf("repo: insert audit: %w", err)
	}
	return nil
}
