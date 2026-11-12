// Package config memuat konfigurasi dari environment variable.
//
// Semua service memakai env (12-factor). Untuk development, .env di root repo
// dimuat otomatis lewat LoadDotEnv.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// LoadDotEnv mencari file .env dari direktori kerja ke atas (maks 5 level)
// lalu memuatnya bila ada. Tidak error bila tidak ditemukan.
func LoadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, ".env")
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

// Get mengembalikan nilai env atau default bila kosong.
func Get(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// MustGet mengembalikan nilai env dan panic bila kosong.
// Hanya dipakai saat startup, bukan di jalur request.
func MustGet(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		panic(fmt.Sprintf("config: environment variable %s wajib diisi", key))
	}
	return v
}

// Duration mem-parse env sebagai durasi (mis. "15m", "720h").
// Nilai tidak valid atau kosong memakai default.
func Duration(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return d
}

// Int mem-parse env sebagai integer. Nilai tidak valid memakai default.
func Int(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// Int64 mem-parse env sebagai int64. Nilai tidak valid memakai default.
func Int64(key string, def int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// Bool mem-parse env sebagai boolean. Nilai tidak valid memakai default.
func Bool(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return b
}

// List mem-parse env yang dipisahkan koma.
func List(key string, def []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
