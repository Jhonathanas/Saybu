package domain

import "testing"

func TestInitialStatus(t *testing.T) {
	cases := map[string]string{
		MethodTransfer: StatusUnpaid,
		MethodCOD:      StatusCODPending,
	}
	for method, want := range cases {
		if got := InitialStatus(method); got != want {
			t.Errorf("InitialStatus(%q) = %q, want %q", method, got, want)
		}
	}
}

func TestIsValidMethod(t *testing.T) {
	for _, m := range []string{MethodTransfer, MethodCOD} {
		if !IsValidMethod(m) {
			t.Errorf("IsValidMethod(%q) = false", m)
		}
	}
	if IsValidMethod("CASH") {
		t.Error("IsValidMethod(CASH) = true, want false")
	}
}

func TestIsValidStatus(t *testing.T) {
	valid := []string{
		StatusUnpaid, StatusWaitingVerification, StatusPaid, StatusRejected,
		StatusCancelled, StatusCODPending, StatusCODPaid,
	}
	for _, s := range valid {
		if !IsValidStatus(s) {
			t.Errorf("IsValidStatus(%q) = false", s)
		}
	}
	if IsValidStatus("NOPE") {
		t.Error("IsValidStatus(NOPE) = true, want false")
	}
}

func TestCanUploadProof(t *testing.T) {
	cases := map[string]bool{
		StatusUnpaid:              true,
		StatusRejected:            true,
		StatusWaitingVerification: false,
		StatusPaid:                false,
		StatusCancelled:           false,
		StatusCODPending:          false,
	}
	for status, want := range cases {
		if got := CanUploadProof(status); got != want {
			t.Errorf("CanUploadProof(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestCanVerify(t *testing.T) {
	if !CanVerify(StatusWaitingVerification) {
		t.Error("CanVerify(WAITING_VERIFICATION) = false")
	}
	if CanVerify(StatusUnpaid) {
		t.Error("CanVerify(UNPAID) = true, want false")
	}
}

func TestIsCancellable(t *testing.T) {
	cases := map[string]bool{
		StatusUnpaid:              true,
		StatusWaitingVerification: true,
		StatusRejected:            true,
		StatusCODPending:          true,
		StatusPaid:                false,
		StatusCancelled:           false,
		StatusCODPaid:             false,
	}
	for status, want := range cases {
		if got := IsCancellable(status); got != want {
			t.Errorf("IsCancellable(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestPhoneMatches(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0812-9876-5432", "081298765432", true},
		{"+62 812 9876 5432", "6281298765432", true},
		{"081298765432", "081200000000", false},
		{"", "", false},
		{"081298765432", "", false},
	}
	for _, c := range cases {
		if got := PhoneMatches(c.a, c.b); got != c.want {
			t.Errorf("PhoneMatches(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestValidationError(t *testing.T) {
	ve := &ValidationError{}
	if err := ve.OrNil(); err != nil {
		t.Fatalf("OrNil kosong = %v, want nil", err)
	}
	ve.Add("field", "pesan")
	if err := ve.OrNil(); err == nil {
		t.Fatal("OrNil dengan error = nil")
	}
	if got := ve.Error(); got == "" {
		t.Fatal("Error() kosong")
	}
}
