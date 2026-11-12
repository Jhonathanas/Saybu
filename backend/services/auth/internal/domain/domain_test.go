package domain

import (
	"errors"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct{ in, want string }{
		{"081234567890", "081234567890"},
		{" 0812-3456-7890 ", "081234567890"},
		{"+62 812-3456-7890", "081234567890"},
		{"6281234567890", "081234567890"},
		{"(0812) 3456 7890", "081234567890"},
	}
	for _, tt := range tests {
		if got := NormalizePhone(tt.in); got != tt.want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestValidatePhone(t *testing.T) {
	valid := []string{"081234567890", "0812345678", "0812345678901"}
	for _, p := range valid {
		if !ValidatePhone(p) {
			t.Errorf("%q harus valid", p)
		}
	}
	invalid := []string{"", "081234567", "08123456789012", "6281234567890", "12345", "0812abc"}
	for _, p := range invalid {
		if ValidatePhone(p) {
			t.Errorf("%q harus tidak valid", p)
		}
	}
}

func TestValidateRegister(t *testing.T) {
	name, phone, email, err := ValidateRegister(" Siti ", "+62 812-3456-7890", " SITI@Example.com ", "rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Siti" || phone != "081234567890" || email != "siti@example.com" {
		t.Fatalf("hasil = %q %q %q", name, phone, email)
	}

	_, _, _, err = ValidateRegister("S", "123", "bukan-email", "pendek")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	fields := map[string]bool{}
	for _, fe := range ve.Errors {
		fields[fe.Field] = true
	}
	for _, want := range []string{"name", "phone", "email", "password"} {
		if !fields[want] {
			t.Errorf("field %q tidak dilaporkan", want)
		}
	}
}

func TestValidateEmailOptional(t *testing.T) {
	if !ValidateEmail("") {
		t.Fatal("email kosong harus valid (opsional)")
	}
	if ValidateEmail("a@b") {
		t.Fatal("a@b harus tidak valid")
	}
}

func TestHasher(t *testing.T) {
	h := NewTestHasher()
	encoded, err := h.Hash("rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if encoded == "rahasia123" {
		t.Fatal("password tidak boleh tersimpan plaintext")
	}
	if !h.Verify(encoded, "rahasia123") {
		t.Fatal("password benar harus lolos")
	}
	if h.Verify(encoded, "salah") {
		t.Fatal("password salah harus gagal")
	}
	if h.Verify("bukan-hash", "rahasia123") {
		t.Fatal("hash invalid harus gagal")
	}
}
