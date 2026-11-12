// Package domain berisi entity dan aturan bisnis auth yang murni
// (tanpa dependensi ke database, gRPC, atau HTTP).
package domain

import (
	"errors"
	"strings"
	"time"
)

// Error domain auth.
var (
	ErrUserExists         = errors.New("user sudah terdaftar")
	ErrUserNotFound       = errors.New("user tidak ditemukan")
	ErrInvalidCredentials = errors.New("kredensial salah")
	ErrInvalidToken       = errors.New("token tidak valid")
	ErrTokenReused        = errors.New("refresh token dipakai ulang")
)

// Role adalah peran user.
type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

// User adalah akun pengguna.
type User struct {
	ID           string
	Name         string
	Phone        string
	Email        *string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

// EmailString mengembalikan email sebagai string (kosong bila nil).
func (u User) EmailString() string {
	if u.Email == nil {
		return ""
	}
	return *u.Email
}

// RefreshToken adalah sesi refresh token yang tersimpan.
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent string
	IP        string
}

// FieldError menjelaskan satu kesalahan validasi.
type FieldError struct {
	Field   string
	Message string
}

// ValidationError mengumpulkan kesalahan validasi per field.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fe.Field+": "+fe.Message)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

// Add menambahkan satu kesalahan field.
func (e *ValidationError) Add(field, message string) *ValidationError {
	e.Errors = append(e.Errors, FieldError{Field: field, Message: message})
	return e
}

// OrNil mengembalikan error atau nil bila tidak ada kesalahan.
func (e *ValidationError) OrNil() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}
