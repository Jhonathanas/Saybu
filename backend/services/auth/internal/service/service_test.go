package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"saybu/pkg/authn"
	"saybu/services/auth/internal/domain"
)

type fakeRepo struct {
	mu     sync.Mutex
	users  map[string]domain.User
	tokens map[string]domain.RefreshToken
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:  map[string]domain.User{},
		tokens: map[string]domain.RefreshToken{},
	}
}

func (f *fakeRepo) CreateUser(_ context.Context, u domain.User) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.users {
		if existing.Phone == u.Phone {
			return domain.User{}, domain.ErrUserExists
		}
		if u.Email != nil && existing.Email != nil && *existing.Email == *u.Email {
			return domain.User{}, domain.ErrUserExists
		}
	}
	u.CreatedAt = time.Now()
	f.users[u.ID] = u
	return u, nil
}

func (f *fakeRepo) GetUserByID(_ context.Context, id string) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeRepo) GetUserByIdentifier(_ context.Context, identifier string) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Phone == identifier || (u.Email != nil && *u.Email == identifier) {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrUserNotFound
}

func (f *fakeRepo) ListUsers(_ context.Context, _ string, limit, offset int) ([]domain.User, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	all := make([]domain.User, 0, len(f.users))
	for _, u := range f.users {
		all = append(all, u)
	}
	if offset >= len(all) {
		return nil, int64(len(all)), nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], int64(len(all)), nil
}

func (f *fakeRepo) CreateRefreshToken(_ context.Context, t domain.RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[t.TokenHash] = t
	return nil
}

func (f *fakeRepo) RotateRefreshToken(_ context.Context, oldHash string, newToken domain.RefreshToken, now time.Time) (domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	old, ok := f.tokens[oldHash]
	if !ok {
		return domain.User{}, domain.ErrInvalidToken
	}
	if old.RevokedAt != nil {
		for hash, t := range f.tokens {
			if t.UserID == old.UserID && t.RevokedAt == nil {
				t.RevokedAt = &now
				f.tokens[hash] = t
			}
		}
		return domain.User{}, domain.ErrTokenReused
	}
	if !old.ExpiresAt.After(now) {
		return domain.User{}, domain.ErrInvalidToken
	}

	old.RevokedAt = &now
	f.tokens[oldHash] = old

	newToken.UserID = old.UserID
	f.tokens[newToken.TokenHash] = newToken

	return f.users[old.UserID], nil
}

func (f *fakeRepo) RevokeRefreshTokenByHash(_ context.Context, hash string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tokens[hash]; ok && t.RevokedAt == nil {
		t.RevokedAt = &now
		f.tokens[hash] = t
	}
	return nil
}

func (f *fakeRepo) RevokeAllRefreshTokens(_ context.Context, userID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, t := range f.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
			f.tokens[hash] = t
		}
	}
	return nil
}

func newTestService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	tokens, err := authn.NewManager(strings.Repeat("s", 32), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	svc, err := New(Options{
		Repo:       repo,
		Hasher:     domain.NewTestHasher(),
		Tokens:     tokens,
		RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo
}

func TestRegisterAndLogin(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	res, err := svc.Register(ctx, RegisterInput{
		Name:     "Siti",
		Phone:    "+62 812-3456-7890",
		Email:    "SITI@example.com",
		Password: "rahasia123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.User.Phone != "081234567890" || res.User.EmailString() != "siti@example.com" {
		t.Fatalf("user = %+v", res.User)
	}
	if res.User.Role != domain.RoleCustomer {
		t.Fatalf("role = %q", res.User.Role)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("token kosong")
	}
	if len(repo.tokens) != 1 {
		t.Fatalf("refresh token tersimpan = %d", len(repo.tokens))
	}

	// Login dengan email (case-insensitive).
	if _, err := svc.Login(ctx, "SITI@EXAMPLE.COM", "rahasia123", "", ""); err != nil {
		t.Fatalf("login email: %v", err)
	}
	// Login dengan nomor HP.
	if _, err := svc.Login(ctx, "081234567890", "rahasia123", "", ""); err != nil {
		t.Fatalf("login phone: %v", err)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	in := RegisterInput{Name: "Siti", Phone: "081234567890", Password: "rahasia123"}
	if _, err := svc.Register(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, in); !errors.Is(err, domain.ErrUserExists) {
		t.Fatalf("err = %v, want ErrUserExists", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Register(context.Background(), RegisterInput{Name: "S", Phone: "123", Password: "x"})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegisterInput{Name: "Siti", Phone: "081234567890", Password: "rahasia123"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, "081234567890", "salah-sekali", "", ""); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := svc.Login(ctx, "089999999999", "rahasia123", "", ""); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestRefreshRotationAndReuse(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()

	res, err := svc.Register(ctx, RegisterInput{Name: "Siti", Phone: "081234567890", Password: "rahasia123"})
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := svc.Refresh(ctx, res.RefreshToken, "ua", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshToken == res.RefreshToken {
		t.Fatal("refresh token tidak berotasi")
	}
	if rotated.AccessToken == "" {
		t.Fatal("access token kosong")
	}

	// Memakai token lama lagi = reuse → semua sesi dicabut.
	if _, err := svc.Refresh(ctx, res.RefreshToken, "", ""); !errors.Is(err, domain.ErrTokenReused) {
		t.Fatalf("err = %v, want ErrTokenReused", err)
	}
	for hash, tok := range repo.tokens {
		if tok.UserID == res.User.ID && tok.RevokedAt == nil {
			t.Fatalf("token %s masih aktif setelah reuse", hash)
		}
	}
}

func TestLogout(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	res, err := svc.Register(ctx, RegisterInput{Name: "Siti", Phone: "081234567890", Password: "rahasia123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(ctx, res.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, res.RefreshToken, "", ""); err == nil {
		t.Fatal("refresh setelah logout harus gagal")
	}
	if err := svc.Logout(ctx, ""); err != nil {
		t.Fatalf("logout token kosong: %v", err)
	}
}

func TestGetUserAndListUsers(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	res, err := svc.Register(ctx, RegisterInput{Name: "Siti", Phone: "081234567890", Password: "rahasia123"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := svc.GetUser(ctx, res.User.ID)
	if err != nil || u.ID != res.User.ID {
		t.Fatalf("GetUser = %+v, %v", u, err)
	}
	if _, err := svc.GetUser(ctx, "tidak-ada"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}

	users, total, page, err := svc.ListUsers(ctx, 0, 500, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || total != 1 || page != 1 {
		t.Fatalf("users=%d total=%d page=%d", len(users), total, page)
	}
}
