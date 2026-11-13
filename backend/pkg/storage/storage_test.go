package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestDisk(t *testing.T) *Disk {
	t.Helper()
	d, err := NewDisk(t.TempDir(), "/uploads")
	if err != nil {
		t.Fatal(err)
	}
	d.now = func() time.Time { return time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC) }
	return d
}

func TestSaveWritesFileAndReturnsURL(t *testing.T) {
	d := newTestDisk(t)
	ctx := context.Background()

	url, err := d.Save(ctx, ".jpg", bytes.NewBufferString("isi-gambar"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/uploads/2026/11/") || !strings.HasSuffix(url, ".jpg") {
		t.Fatalf("url = %q", url)
	}

	full, err := d.Path(url)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "isi-gambar" {
		t.Fatalf("isi = %q", got)
	}
}

func TestSaveNormalizesExtension(t *testing.T) {
	d := newTestDisk(t)
	url, err := d.Save(context.Background(), "PNG", bytes.NewBufferString("x"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(url, ".png") {
		t.Fatalf("url = %q", url)
	}
}

func TestSaveRejectsUnknownExtension(t *testing.T) {
	d := newTestDisk(t)
	if _, err := d.Save(context.Background(), ".gif", bytes.NewBufferString("x")); err == nil {
		t.Fatal("harus menolak ekstensi tak dikenal")
	}
}

func TestDeleteRemovesFile(t *testing.T) {
	d := newTestDisk(t)
	ctx := context.Background()

	url, err := d.Save(ctx, ".webp", bytes.NewBufferString("x"))
	if err != nil {
		t.Fatal(err)
	}
	full, _ := d.Path(url)

	if err := d.Delete(ctx, url); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		t.Fatalf("berkas masih ada: %v", err)
	}
	// Menghapus berkas yang sudah tidak ada bukan error.
	if err := d.Delete(ctx, url); err != nil {
		t.Fatalf("delete kedua = %v", err)
	}
}

func TestPathRejectsTraversal(t *testing.T) {
	d := newTestDisk(t)
	cases := []string{"", "/uploads/../../etc/passwd", "/etc/passwd", "/uploads/../secret.txt", "/uploads/sub/../../secret"}
	for _, c := range cases {
		if _, err := d.Path(c); err == nil {
			t.Fatalf("Path(%q) seharusnya error", c)
		}
	}
}

func TestNewDiskRequiresDir(t *testing.T) {
	if _, err := NewDisk("  ", "/uploads"); err == nil {
		t.Fatal("harus error bila direktori kosong")
	}
}

func TestNewDiskCreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "uploads")
	if _, err := NewDisk(dir, "/uploads"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("direktori tidak dibuat: %v", err)
	}
}
