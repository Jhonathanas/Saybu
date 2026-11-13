package domain

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Bayam Segar", "bayam-segar"},
		{"  Tomat Merah  ", "tomat-merah"},
		{"Jeruk Nipis 1kg", "jeruk-nipis-1kg"},
		{"Sawi--Hijau", "sawi-hijau"},
		{"!!!", ""},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateProduct(t *testing.T) {
	cases := []struct {
		name     string
		in       ProductInput
		wantErr  bool
		wantSlug string
	}{
		{
			name:     "valid dengan slug turunan",
			in:       ProductInput{Name: "Bayam Segar", Unit: UnitIkat, Price: 5000, Stock: 40},
			wantSlug: "bayam-segar",
		},
		{
			name:     "unit default pcs bila kosong",
			in:       ProductInput{Name: "Pisang", Price: 1000},
			wantSlug: "pisang",
		},
		{
			name:    "nama terlalu pendek",
			in:      ProductInput{Name: "A", Unit: UnitPcs, Price: 1},
			wantErr: true,
		},
		{
			name:    "unit tidak dikenal",
			in:      ProductInput{Name: "Tomat", Unit: "liter", Price: 1},
			wantErr: true,
		},
		{
			name:    "harga negatif",
			in:      ProductInput{Name: "Tomat", Unit: UnitKg, Price: -1},
			wantErr: true,
		},
		{
			name:    "slug tidak valid",
			in:      ProductInput{Name: "Tomat", Slug: "Tomat Merah", Unit: UnitKg, Price: 1},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := ValidateProduct(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.wantSlug != "" && out.Slug != c.wantSlug {
				t.Fatalf("slug = %q, want %q", out.Slug, c.wantSlug)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	kg := Product{Unit: UnitKg}
	ApplyDefaults(&kg)
	if kg.MinQty != 100 || kg.StepQty != 100 {
		t.Fatalf("kg defaults = %d/%d, want 100/100", kg.MinQty, kg.StepQty)
	}

	ikat := Product{Unit: UnitIkat}
	ApplyDefaults(&ikat)
	if ikat.MinQty != 1 || ikat.StepQty != 1 {
		t.Fatalf("ikat defaults = %d/%d, want 1/1", ikat.MinQty, ikat.StepQty)
	}
}

func TestDisplayQty(t *testing.T) {
	cases := []struct {
		unit string
		qty  int64
		want string
	}{
		{UnitKg, 1000, "1 kg"},
		{UnitKg, 1500, "1,5 kg"},
		{UnitKg, 250, "0,25 kg"},
		{UnitKg, 0, "0 kg"},
		{UnitIkat, 2, "2 ikat"},
		{UnitPcs, 3, "3 pcs"},
	}
	for _, c := range cases {
		if got := DisplayQty(c.unit, c.qty); got != c.want {
			t.Errorf("DisplayQty(%q, %d) = %q, want %q", c.unit, c.qty, got, c.want)
		}
	}
}

func TestIsKg(t *testing.T) {
	if !IsKg(UnitKg) {
		t.Fatal("kg harus true")
	}
	if IsKg(UnitIkat) {
		t.Fatal("ikat harus false")
	}
}
