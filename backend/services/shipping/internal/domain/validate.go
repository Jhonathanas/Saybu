package domain

import (
	"strings"
	"unicode/utf8"
)

// ZoneInput adalah input pembuatan/pembaruan zona.
type ZoneInput struct {
	ID              string
	Name            string
	Areas           []string
	Fee             int64
	FreeMinSubtotal int64
	IsActive        bool
}

// ValidateZone memvalidasi input zona dan mengembalikan versi rapi.
func ValidateZone(in ZoneInput) (ZoneInput, error) {
	ve := &ValidationError{}

	name := strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(name); n < 2 || n > 120 {
		ve.Add("name", "Nama zona minimal 2 dan maksimal 120 karakter.")
	}
	in.Name = name

	in.Areas = cleanAreas(in.Areas)

	if in.Fee < 0 {
		ve.Add("fee", "Ongkir tidak boleh negatif.")
	}
	if in.FreeMinSubtotal < 0 {
		ve.Add("free_min_subtotal", "Minimum gratis ongkir tidak boleh negatif.")
	}
	return in, ve.OrNil()
}

// cleanAreas memangkas spasi, membuang entri kosong, dan menghapus duplikat
// (tanpa membedakan huruf besar/kecil) sambil mempertahankan urutan.
func cleanAreas(areas []string) []string {
	out := make([]string, 0, len(areas))
	seen := make(map[string]bool, len(areas))
	for _, a := range areas {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		key := strings.ToLower(a)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
	}
	return out
}
