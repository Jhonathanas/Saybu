package money

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		amount int64
		want   string
	}{
		{0, "Rp0"},
		{500, "Rp500"},
		{5000, "Rp5.000"},
		{75000, "Rp75.000"},
		{1234567890, "Rp1.234.567.890"},
		{-1500, "-Rp1.500"},
	}
	for _, tt := range tests {
		if got := Format(tt.amount); got != tt.want {
			t.Errorf("Format(%d) = %q, want %q", tt.amount, got, tt.want)
		}
	}
}

func TestRupiahAndSum(t *testing.T) {
	r := Rupiah(85000)
	if got := r.Format(); got != "Rp85.000" {
		t.Fatalf("Format = %q", got)
	}
	if got := Sum(75000, 10000); got != 85000 {
		t.Fatalf("Sum = %d", got)
	}
	if Valid(-1) {
		t.Fatal("Valid(-1) harus false")
	}
}
