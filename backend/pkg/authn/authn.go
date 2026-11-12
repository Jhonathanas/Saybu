// Package authn menyediakan JWT access token dan refresh token yang dipakai
// auth service (menandatangani) dan gateway (memverifikasi).
package authn

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken dikembalikan bila token tidak valid, kedaluwarsa, atau
// tanda tangannya salah.
var ErrInvalidToken = errors.New("token tidak valid")

const issuer = "saybu-auth"

// Claims adalah isi access token.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Manager membuat dan memverifikasi access token.
type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewManager membuat Manager. Secret minimal 32 karakter.
func NewManager(secret string, accessTTL time.Duration) (*Manager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("authn: JWT_SECRET minimal 32 karakter")
	}
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	return &Manager{secret: []byte(secret), accessTTL: accessTTL}, nil
}

// AccessTTL mengembalikan masa berlaku access token.
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// NewAccessToken membuat access token HS256 untuk user.
func (m *Manager) NewAccessToken(userID, role string) (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(m.accessTTL)
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("authn: tanda tangan token: %w", err)
	}
	return signed, expires, nil
}

// Parse memverifikasi access token dan mengembalikan claims.
func (m *Manager) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Subject == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// NewRefreshToken membuat token acak 256-bit (raw) dan hash SHA-256-nya.
// Yang disimpan di database hanya hash.
func NewRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("authn: buat refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken mengembalikan hash SHA-256 hex dari refresh token.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
