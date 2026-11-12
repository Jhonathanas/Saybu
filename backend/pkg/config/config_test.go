package config

import (
	"os"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	t.Setenv("SAYBU_TEST_GET", "  hello  ")
	if got := Get("SAYBU_TEST_GET", "def"); got != "hello" {
		t.Fatalf("Get = %q, want hello", got)
	}
	if got := Get("SAYBU_TEST_MISSING", "def"); got != "def" {
		t.Fatalf("Get default = %q, want def", got)
	}
}

func TestDuration(t *testing.T) {
	t.Setenv("SAYBU_TEST_DUR", "15m")
	if got := Duration("SAYBU_TEST_DUR", time.Hour); got != 15*time.Minute {
		t.Fatalf("Duration = %v, want 15m", got)
	}
	t.Setenv("SAYBU_TEST_DUR", "bukan-durasi")
	if got := Duration("SAYBU_TEST_DUR", time.Hour); got != time.Hour {
		t.Fatalf("Duration invalid = %v, want default", got)
	}
}

func TestIntBool(t *testing.T) {
	t.Setenv("SAYBU_TEST_INT", "42")
	if got := Int("SAYBU_TEST_INT", 1); got != 42 {
		t.Fatalf("Int = %d, want 42", got)
	}
	t.Setenv("SAYBU_TEST_INT", "x")
	if got := Int("SAYBU_TEST_INT", 7); got != 7 {
		t.Fatalf("Int invalid = %d, want 7", got)
	}

	t.Setenv("SAYBU_TEST_BOOL", "true")
	if got := Bool("SAYBU_TEST_BOOL", false); !got {
		t.Fatal("Bool = false, want true")
	}
}

func TestList(t *testing.T) {
	t.Setenv("SAYBU_TEST_LIST", "a, b ,, c")
	got := List("SAYBU_TEST_LIST", nil)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("List = %#v", got)
	}
}

func TestLoadDotEnvMissing(t *testing.T) {
	// Tidak boleh panic bila .env tidak ada.
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	LoadDotEnv()
}
