//go:build integration

package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/authn"
	"saybu/pkg/ids"
	"saybu/services/auth/internal/domain"
)

// TestRepoIntegration membutuhkan Postgres dengan migrasi auth sudah dijalankan.
//
//	AUTH_DB_DSN=... go test -tags=integration ./services/auth/internal/repo/...
func TestRepoIntegration(t *testing.T) {
	dsn := os.Getenv("AUTH_DB_DSN")
	if dsn == "" {
		t.Skip("AUTH_DB_DSN tidak diset")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres tidak tersedia: %v", err)
	}

	r := New(pool)
	// Pakai bagian acak (akhir) UUIDv7 agar unik antar-run, bukan timestamp.
	raw := strings.ReplaceAll(ids.New(), "-", "")
	n, err := strconv.ParseUint(raw[len(raw)-8:], 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%08d", n%100000000)
	phone := "0812" + suffix
	email := "test-" + suffix + "@example.com"

	hash, err := domain.NewTestHasher().Hash("rahasia123")
	if err != nil {
		t.Fatal(err)
	}

	user, err := r.CreateUser(ctx, domain.User{
		ID:           ids.New(),
		Name:         "Test Repo",
		Phone:        phone,
		Email:        &email,
		PasswordHash: hash,
		Role:         domain.RoleCustomer,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	if _, err := r.CreateUser(ctx, domain.User{
		ID: ids.New(), Name: "Dup", Phone: phone, PasswordHash: hash, Role: domain.RoleCustomer,
	}); !errors.Is(err, domain.ErrUserExists) {
		t.Fatalf("duplikat: err = %v", err)
	}

	got, err := r.GetUserByIdentifier(ctx, strings.ToUpper(email))
	if err != nil || got.ID != user.ID {
		t.Fatalf("GetUserByIdentifier = %+v, %v", got, err)
	}

	_, newHash, err := authn.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := r.CreateRefreshToken(ctx, domain.RefreshToken{
		ID: ids.New(), UserID: user.ID, TokenHash: newHash,
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	_, rotatedHash, err := authn.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := r.RotateRefreshToken(ctx, newHash, domain.RefreshToken{
		ID: ids.New(), TokenHash: rotatedHash, ExpiresAt: now.Add(time.Hour),
	}, now)
	if err != nil || rotated.ID != user.ID {
		t.Fatalf("rotate = %+v, %v", rotated, err)
	}

	// Reuse token lama harus mencabut semua sesi.
	if _, err := r.RotateRefreshToken(ctx, newHash, domain.RefreshToken{
		ID: ids.New(), TokenHash: rotatedHash + "x", ExpiresAt: now.Add(time.Hour),
	}, now); !errors.Is(err, domain.ErrTokenReused) {
		t.Fatalf("reuse: err = %v", err)
	}

	users, total, err := r.ListUsers(ctx, "Test Repo", 10, 0)
	if err != nil || total < 1 || len(users) < 1 {
		t.Fatalf("ListUsers = %d, %d, %v", len(users), total, err)
	}
}
