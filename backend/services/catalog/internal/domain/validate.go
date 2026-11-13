package domain

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Slugify membuat slug dari teks bebas: huruf kecil, non-alfanumerik jadi "-".
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// ValidateName memvalidasi nama produk/kategori.
func ValidateName(name string) (string, bool) {
	n := strings.TrimSpace(name)
	length := utf8.RuneCountInString(n)
	if length < 2 || length > 120 {
		return n, false
	}
	return n, true
}

// ValidateSlug memvalidasi slug; bila kosong diturunkan dari fallback (nama).
func ValidateSlug(slug, fallback string) (string, bool) {
	s := strings.TrimSpace(slug)
	if s == "" {
		s = Slugify(fallback)
	}
	if len(s) < 2 || len(s) > 120 || !slugRe.MatchString(s) {
		return s, false
	}
	return s, true
}

// ValidUnit memeriksa apakah satuan dikenal.
func ValidUnit(unit string) bool {
	for _, u := range AllowedUnits {
		if u == unit {
			return true
		}
	}
	return false
}

// ProductInput adalah input pembuatan produk (sudah divalidasi).
type ProductInput struct {
	Name        string
	Slug        string
	CategoryID  string
	Description string
	Unit        string
	Price       int64
	Stock       int64
	MinQty      int64
	StepQty     int64
	IsActive    bool
	Images      []string
}

// ValidateProduct memvalidasi input produk dan mengembalikan versi rapi.
func ValidateProduct(in ProductInput) (ProductInput, error) {
	ve := &ValidationError{}

	name, ok := ValidateName(in.Name)
	if !ok {
		ve.Add("name", "Nama minimal 2 dan maksimal 120 karakter.")
	}
	in.Name = name

	slug, ok := ValidateSlug(in.Slug, in.Name)
	if !ok {
		ve.Add("slug", "Slug hanya boleh huruf kecil, angka, dan tanda hubung.")
	}
	in.Slug = slug

	if in.Unit == "" {
		in.Unit = UnitPcs
	}
	if !ValidUnit(in.Unit) {
		ve.Add("unit", "Satuan harus salah satu dari: "+strings.Join(AllowedUnits, ", ")+".")
	}

	if in.Price < 0 {
		ve.Add("price", "Harga tidak boleh negatif.")
	}
	if in.Stock < 0 {
		ve.Add("stock", "Stok tidak boleh negatif.")
	}
	if in.MinQty < 0 {
		ve.Add("min_qty", "Qty minimum tidak boleh negatif.")
	}
	if in.StepQty < 0 {
		ve.Add("step_qty", "Langkah qty tidak boleh negatif.")
	}

	in.Description = strings.TrimSpace(in.Description)
	in.CategoryID = strings.TrimSpace(in.CategoryID)

	return in, ve.OrNil()
}

// ApplyDefaults mengisi min/step qty bila belum diisi, sesuai satuan.
func ApplyDefaults(p *Product) {
	if p.MinQty <= 0 {
		p.MinQty = MinDefault(p.Unit)
	}
	if p.StepQty <= 0 {
		p.StepQty = StepDefault(p.Unit)
	}
}
