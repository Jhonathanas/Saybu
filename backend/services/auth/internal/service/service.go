// Package service berisi usecase auth: register, login, rotasi refresh, logout.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"saybu/pkg/authn"
	"saybu/pkg/ids"
	"saybu/services/auth/internal/domain"
)

// Repo adalah kebutuhan service terhadap database.
type Repo interface {
	CreateUser(ctx context.Context, u domain.User) (domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	GetUserByIdentifier(ctx context.Context, identifier string) (domain.User, error)
	ListUsers(ctx context.Context, query string, limit, offset int) ([]domain.User, int64, error)
	CreateRefreshToken(ctx context.Context, t domain.RefreshToken) error
	RotateRefreshToken(ctx context.Context, oldHash string, newToken domain.RefreshToken, now time.Time) (domain.User, error)
	RevokeRefreshTokenByHash(ctx context.Context, hash string, now time.Time) error
	RevokeAllRefreshTokens(ctx context.Context, userID string, now time.Time) error
}

// Options konfigurasi Service.
type Options struct {
	Repo       Repo
	Hasher     *domain.Hasher
	Tokens     *authn.Manager
	RefreshTTL time.Duration
	Now        func() time.Time
}

// Service adalah usecase auth.
type Service struct {
	repo       Repo
	hasher     *domain.Hasher
	tokens     *authn.Manager
	refreshTTL time.Duration
	now        func() time.Time
	dummyHash  string
}

// New membuat Service.
func New(opts Options) (*Service, error) {
	if opts.Repo == nil || opts.Hasher == nil || opts.Tokens == nil {
		return nil, errors.New("service: repo, hasher, dan token manager wajib")
	}
	if opts.RefreshTTL <= 0 {
		opts.RefreshTTL = 30 * 24 * time.Hour
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	dummy, err := opts.Hasher.Hash("saybu-dummy-password")
	if err != nil {
		return nil, fmt.Errorf("service: siapkan dummy hash: %w", err)
	}
	return &Service{
		repo:       opts.Repo,
		hasher:     opts.Hasher,
		tokens:     opts.Tokens,
		refreshTTL: opts.RefreshTTL,
		now:        opts.Now,
		dummyHash:  dummy,
	}, nil
}

// RegisterInput adalah input registrasi.
type RegisterInput struct {
	Name      string
	Phone     string
	Email     string
	Password  string
	UserAgent string
	IP        string
}

// AuthResult adalah hasil autentikasi.
type AuthResult struct {
	User             domain.User
	AccessToken      string
	AccessExpiresIn  time.Duration
	RefreshToken     string
	RefreshExpiresIn time.Duration
}

// Register membuat akun customer baru lalu menerbitkan token.
func (s *Service) Register(ctx context.Context, in RegisterInput) (AuthResult, error) {
	name, phone, email, err := domain.ValidateRegister(in.Name, in.Phone, in.Email, in.Password)
	if err != nil {
		return AuthResult{}, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return AuthResult{}, err
	}

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	user, err := s.repo.CreateUser(ctx, domain.User{
		ID:           ids.New(),
		Name:         name,
		Phone:        phone,
		Email:        emailPtr,
		PasswordHash: hash,
		Role:         domain.RoleCustomer,
	})
	if err != nil {
		return AuthResult{}, err
	}
	return s.issue(ctx, user, in.UserAgent, in.IP)
}

// Login memverifikasi identifier (HP/email) dan password.
func (s *Service) Login(ctx context.Context, identifier, password, userAgent, ip string) (AuthResult, error) {
	id := strings.TrimSpace(identifier)
	if id == "" || password == "" {
		return AuthResult{}, (&domain.ValidationError{}).
			Add("identifier", "Nomor HP atau email wajib diisi.").
			Add("password", "Password wajib diisi.")
	}
	if strings.Contains(id, "@") {
		id = domain.NormalizeEmail(id)
	} else {
		id = domain.NormalizePhone(id)
	}

	user, err := s.repo.GetUserByIdentifier(ctx, id)
	if errors.Is(err, domain.ErrUserNotFound) {
		// Tetap verifikasi hash dummy agar waktu respons tidak membocorkan
		// apakah akun ada atau tidak.
		s.hasher.Verify(s.dummyHash, password)
		return AuthResult{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return AuthResult{}, err
	}
	if !s.hasher.Verify(user.PasswordHash, password) {
		return AuthResult{}, domain.ErrInvalidCredentials
	}
	return s.issue(ctx, user, userAgent, ip)
}

// Refresh merotasi refresh token dan menerbitkan access token baru.
func (s *Service) Refresh(ctx context.Context, rawToken, userAgent, ip string) (AuthResult, error) {
	if strings.TrimSpace(rawToken) == "" {
		return AuthResult{}, domain.ErrInvalidToken
	}

	newRaw, newHash, err := authn.NewRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}

	now := s.now()
	user, err := s.repo.RotateRefreshToken(ctx, authn.HashRefreshToken(rawToken), domain.RefreshToken{
		ID:        ids.New(),
		TokenHash: newHash,
		ExpiresAt: now.Add(s.refreshTTL),
		UserAgent: userAgent,
		IP:        ip,
	}, now)
	if err != nil {
		return AuthResult{}, err
	}

	access, expiresAt, err := s.tokens.NewAccessToken(user.ID, string(user.Role))
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:             user,
		AccessToken:      access,
		AccessExpiresIn:  expiresAt.Sub(s.now()),
		RefreshToken:     newRaw,
		RefreshExpiresIn: s.refreshTTL,
	}, nil
}

// Logout mencabut refresh token. Token kosong dianggap sudah logout.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	return s.repo.RevokeRefreshTokenByHash(ctx, authn.HashRefreshToken(rawToken), s.now())
}

// GetUser mengambil profil user.
func (s *Service) GetUser(ctx context.Context, id string) (domain.User, error) {
	if strings.TrimSpace(id) == "" {
		return domain.User{}, (&domain.ValidationError{}).Add("id", "ID user wajib diisi.")
	}
	return s.repo.GetUserByID(ctx, id)
}

// ListUsers mengambil daftar user (admin).
func (s *Service) ListUsers(ctx context.Context, page, limit int, query string) ([]domain.User, int64, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	users, total, err := s.repo.ListUsers(ctx, query, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, 0, err
	}
	return users, total, page, nil
}

func (s *Service) issue(ctx context.Context, user domain.User, userAgent, ip string) (AuthResult, error) {
	access, expiresAt, err := s.tokens.NewAccessToken(user.ID, string(user.Role))
	if err != nil {
		return AuthResult{}, err
	}

	raw, hash, err := authn.NewRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.repo.CreateRefreshToken(ctx, domain.RefreshToken{
		ID:        ids.New(),
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: s.now().Add(s.refreshTTL),
		UserAgent: userAgent,
		IP:        ip,
	}); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:             user,
		AccessToken:      access,
		AccessExpiresIn:  expiresAt.Sub(s.now()),
		RefreshToken:     raw,
		RefreshExpiresIn: s.refreshTTL,
	}, nil
}
