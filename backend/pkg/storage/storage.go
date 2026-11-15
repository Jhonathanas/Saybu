// Package storage menyimpan file unggahan (mis. gambar produk) dan
// mengembalikan URL publik yang dipakai frontend.
//
// Implementasi saat ini menulis ke disk lokal (`UPLOAD_DIR`); antarmuka
// sengaja tipis agar mudah diganti object storage pada M7.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"saybu/pkg/ids"
)

// ErrInvalidPath dikembalikan bila URL tidak berada di dalam direktori storage.
var ErrInvalidPath = errors.New("storage: path di luar direktori unggahan")

// AllowedExt adalah ekstensi gambar yang diterima.
var AllowedExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// AllowedProofExt adalah ekstensi bukti pembayaran yang diterima
// (gambar atau PDF).
var AllowedProofExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".pdf":  true,
}

// Storage menyimpan dan menghapus berkas unggahan.
type Storage interface {
	// Save menyimpan isi berkas dengan ekstensi tertentu dan mengembalikan
	// URL publiknya (mis. "/uploads/2026/11/<uuid>.jpg").
	Save(ctx context.Context, ext string, r io.Reader) (string, error)
	// Delete menghapus berkas berdasarkan URL publik dari Save.
	// Berkas yang sudah tidak ada dianggap berhasil.
	Delete(ctx context.Context, url string) error
}

// Disk menyimpan berkas di direktori lokal.
type Disk struct {
	dir       string
	urlPrefix string
	now       func() time.Time
}

// NewDisk membuat penyimpanan disk. urlPrefix dipakai untuk membentuk URL
// publik (mis. "/uploads").
func NewDisk(dir, urlPrefix string) (*Disk, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("storage: direktori wajib diisi")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve direktori: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: buat direktori: %w", err)
	}
	if urlPrefix == "" {
		urlPrefix = "/uploads"
	}
	return &Disk{dir: abs, urlPrefix: strings.TrimRight(urlPrefix, "/"), now: time.Now}, nil
}

// Dir mengembalikan direktori akar penyimpanan (dipakai untuk melayani file statis).
func (d *Disk) Dir() string { return d.dir }

// Save menulis berkas ke {dir}/{yyyy}/{mm}/{uuid}{ext}.
func (d *Disk) Save(ctx context.Context, ext string, r io.Reader) (string, error) {
	return d.SaveWith(ctx, ext, r, AllowedExt)
}

// SaveWith menulis berkas dengan allow-list ekstensi tertentu. Dipakai untuk
// unggahan non-gambar (mis. bukti transfer PDF).
func (d *Disk) SaveWith(ctx context.Context, ext string, r io.Reader, allowed map[string]bool) (string, error) {
	ext = normalizeExt(ext)
	if !allowed[ext] {
		return "", fmt.Errorf("storage: ekstensi %q tidak didukung", ext)
	}

	now := d.now().In(jakarta)
	relDir := path.Join(now.Format("2006"), now.Format("01"))
	rel := path.Join(relDir, ids.New()+ext)

	full := filepath.Join(d.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("storage: buat subdirektori: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(full), "upload-*.tmp")
	if err != nil {
		return "", fmt.Errorf("storage: buat berkas sementara: %w", err)
	}
	tmpName := tmp.Name()
	// Bersihkan berkas sementara bila gagal di tengah jalan.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return "", fmt.Errorf("storage: tulis berkas: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("storage: tutup berkas: %w", err)
	}
	if err := os.Rename(tmpName, full); err != nil {
		return "", fmt.Errorf("storage: pindahkan berkas: %w", err)
	}
	return d.urlPrefix + "/" + path.Join(relDir, path.Base(rel)), nil
}

// Delete menghapus berkas dari URL publiknya.
func (d *Disk) Delete(_ context.Context, url string) error {
	full, err := d.Path(url)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("storage: hapus berkas: %w", err)
	}
	return nil
}

// Path mengubah URL publik menjadi path berkas absolut. Menolak URL di luar
// direktori penyimpanan (mencegah path traversal).
func (d *Disk) Path(url string) (string, error) {
	u := strings.TrimSpace(url)
	if !strings.HasPrefix(u, d.urlPrefix+"/") {
		return "", ErrInvalidPath
	}
	rel := strings.TrimPrefix(u, d.urlPrefix+"/")
	if rel == "" {
		return "", ErrInvalidPath
	}
	full := filepath.Join(d.dir, filepath.FromSlash(path.Clean(rel)))
	relCheck, err := filepath.Rel(d.dir, full)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	return full, nil
}

var jakarta = time.FixedZone("WIB", 7*60*60)

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}
