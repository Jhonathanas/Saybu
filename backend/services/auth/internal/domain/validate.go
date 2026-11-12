package domain

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

var phoneRe = regexp.MustCompile(`^08[0-9]{8,11}$`)

// NormalizePhone membersihkan spasi/tanda hubung dan mengubah +62/62 menjadi 0.
func NormalizePhone(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	switch {
	case strings.HasPrefix(s, "+62"):
		s = "0" + s[3:]
	case strings.HasPrefix(s, "62") && len(s) > 2 && s[2] == '8':
		s = "0" + s[2:]
	}
	return s
}

// ValidatePhone memeriksa format nomor HP Indonesia (08xxxxxxxxxx).
func ValidatePhone(phone string) bool {
	return phoneRe.MatchString(phone)
}

// NormalizeEmail merapikan email (trim + lowercase).
func NormalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ValidateEmail memeriksa format email. String kosong dianggap valid
// karena email opsional.
func ValidateEmail(email string) bool {
	if email == "" {
		return true
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	if !strings.EqualFold(addr.Address, email) {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	domainPart := email[at+1:]
	dot := strings.LastIndex(domainPart, ".")
	return dot > 0 && dot < len(domainPart)-1 && len(domainPart)-dot-1 >= 2
}

// ValidateName memvalidasi nama dan mengembalikan versi rapi.
func ValidateName(name string) (string, bool) {
	n := strings.TrimSpace(name)
	length := utf8.RuneCountInString(n)
	if length < 2 || length > 100 {
		return n, false
	}
	return n, true
}

// ValidatePassword memastikan password minimal 8 dan maksimal 128 karakter.
func ValidatePassword(password string) bool {
	length := utf8.RuneCountInString(password)
	return length >= 8 && length <= 128
}

// ValidateRegister memvalidasi seluruh input registrasi.
func ValidateRegister(name, phone, email, password string) (cleanName, cleanPhone, cleanEmail string, err error) {
	ve := &ValidationError{}

	cleanName, ok := ValidateName(name)
	if !ok {
		ve.Add("name", "Nama minimal 2 dan maksimal 100 karakter.")
	}

	cleanPhone = NormalizePhone(phone)
	if !ValidatePhone(cleanPhone) {
		ve.Add("phone", "Nomor HP harus diawali 08 dan terdiri dari 10-13 digit.")
	}

	cleanEmail = NormalizeEmail(email)
	if !ValidateEmail(cleanEmail) {
		ve.Add("email", "Format email tidak valid.")
	}

	if !ValidatePassword(password) {
		ve.Add("password", "Password minimal 8 dan maksimal 128 karakter.")
	}

	return cleanName, cleanPhone, cleanEmail, ve.OrNil()
}
