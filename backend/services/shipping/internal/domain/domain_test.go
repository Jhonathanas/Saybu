package domain

import (
	"errors"
	"testing"
)

func TestComputeQuote(t *testing.T) {
	zone := Zone{ID: "z1", Name: "Kec. Sukamaju", Fee: 10000, FreeMinSubtotal: 150000}

	tests := []struct {
		name      string
		subtotal  int64
		wantFee   int64
		wantFree  bool
		wantTotal int64
	}{
		{name: "di bawah ambang gratis", subtotal: 75000, wantFee: 10000, wantFree: false, wantTotal: 85000},
		{name: "tepat di ambang gratis", subtotal: 150000, wantFee: 0, wantFree: true, wantTotal: 150000},
		{name: "di atas ambang gratis", subtotal: 200000, wantFee: 0, wantFree: true, wantTotal: 200000},
		{name: "subtotal nol tetap kena ongkir", subtotal: 0, wantFee: 10000, wantFree: false, wantTotal: 10000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := ComputeQuote(zone, tc.subtotal)
			if q.Fee != tc.wantFee || q.FreeShipping != tc.wantFree || q.Total != tc.wantTotal {
				t.Fatalf("quote = %+v", q)
			}
			if q.ZoneID != "z1" || q.ZoneName != "Kec. Sukamaju" || q.Subtotal != tc.subtotal {
				t.Fatalf("quote = %+v", q)
			}
		})
	}
}

func TestComputeQuoteNoFreeThreshold(t *testing.T) {
	zone := Zone{ID: "z1", Name: "Ambil di tempat", Fee: 0, FreeMinSubtotal: 0}
	q := ComputeQuote(zone, 999999)
	if q.FreeShipping || q.Fee != 0 || q.Total != 999999 {
		t.Fatalf("quote = %+v", q)
	}
}

func TestValidateZone(t *testing.T) {
	in := ZoneInput{
		Name:            "  Kec. Sukamaju  ",
		Areas:           []string{" Sukamaju ", "", "sukamaju", "Sukamulya"},
		Fee:             10000,
		FreeMinSubtotal: 150000,
	}
	clean, err := ValidateZone(in)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Name != "Kec. Sukamaju" {
		t.Fatalf("name = %q", clean.Name)
	}
	if len(clean.Areas) != 2 || clean.Areas[0] != "Sukamaju" || clean.Areas[1] != "Sukamulya" {
		t.Fatalf("areas = %+v", clean.Areas)
	}
}

func TestValidateZoneErrors(t *testing.T) {
	_, err := ValidateZone(ZoneInput{Name: "A", Fee: -1, FreeMinSubtotal: -5})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if len(ve.Errors) != 3 {
		t.Fatalf("errors = %+v", ve.Errors)
	}
}
