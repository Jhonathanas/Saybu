package domain

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{StatusPendingPayment, StatusWaitingVerification, true},
		{StatusPendingPayment, StatusPaid, true},
		{StatusPendingPayment, StatusCancelled, true},
		{StatusPendingPayment, StatusExpired, true},
		{StatusPendingPayment, StatusPreparing, false},
		{StatusPendingPayment, StatusShipped, false},
		{StatusWaitingVerification, StatusPaid, true},
		{StatusWaitingVerification, StatusPendingPayment, true},
		{StatusWaitingVerification, StatusCancelled, true},
		{StatusPaid, StatusPreparing, true},
		{StatusPaid, StatusCancelled, true},
		{StatusPaid, StatusShipped, false},
		{StatusPreparing, StatusShipped, true},
		{StatusPreparing, StatusReadyPickup, true},
		{StatusPreparing, StatusCancelled, true},
		{StatusShipped, StatusCompleted, true},
		{StatusReadyPickup, StatusCompleted, true},
		{StatusCompleted, StatusCancelled, false},
		{StatusCancelled, StatusPaid, false},
		{StatusExpired, StatusPaid, false},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestIsValidStatusAndTerminal(t *testing.T) {
	for _, s := range AllStatuses {
		if !IsValidStatus(s) {
			t.Errorf("status %s harus valid", s)
		}
	}
	if IsValidStatus("NGACO") {
		t.Error("status tak dikenal tidak boleh valid")
	}
	for _, s := range []string{StatusCompleted, StatusCancelled, StatusExpired} {
		if !IsTerminal(s) {
			t.Errorf("%s harus terminal", s)
		}
	}
	if IsTerminal(StatusPendingPayment) {
		t.Error("PENDING_PAYMENT bukan terminal")
	}
}

func TestLineTotal(t *testing.T) {
	tests := []struct {
		unit       string
		price, qty int64
		want       int64
	}{
		{"ikat", 5000, 2, 10000},
		{"pcs", 3000, 5, 15000},
		{"kg", 15000, 1000, 15000}, // 1 kg
		{"kg", 15000, 500, 7500},   // 0.5 kg
		{"kg", 25000, 2000, 50000}, // 2 kg
		{"kg", 12000, 250, 3000},   // 0.25 kg
		{"kg", 12345, 100, 1235},   // pembulatan
	}
	for _, tt := range tests {
		if got := LineTotal(tt.unit, tt.price, tt.qty); got != tt.want {
			t.Errorf("LineTotal(%s, %d, %d) = %d, want %d", tt.unit, tt.price, tt.qty, got, tt.want)
		}
	}
}

func TestDisplayQty(t *testing.T) {
	tests := []struct {
		unit string
		qty  int64
		want string
	}{
		{"ikat", 2, "2 ikat"},
		{"kg", 1000, "1 kg"},
		{"kg", 500, "0,5 kg"},
		{"kg", 1500, "1,5 kg"},
		{"kg", 250, "0,25 kg"},
	}
	for _, tt := range tests {
		if got := DisplayQty(tt.unit, tt.qty); got != tt.want {
			t.Errorf("DisplayQty(%s, %d) = %q, want %q", tt.unit, tt.qty, got, tt.want)
		}
	}
}

func TestPhoneMatches(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"081234567890", "0812-3456-7890", true},
		{"+62 812 3456 7890", "081234567890", false},
		{"", "0812", false},
		{"0812", "0813", false},
	}
	for _, tt := range tests {
		if got := PhoneMatches(tt.a, tt.b); got != tt.want {
			t.Errorf("PhoneMatches(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCancellableByCustomer(t *testing.T) {
	for _, s := range []string{StatusPendingPayment, StatusWaitingVerification, StatusPaid} {
		if !CancellableByCustomer(s) {
			t.Errorf("%s harus bisa dibatalkan pembeli", s)
		}
	}
	for _, s := range []string{StatusPreparing, StatusShipped, StatusCompleted, StatusCancelled, StatusExpired} {
		if CancellableByCustomer(s) {
			t.Errorf("%s tidak boleh dibatalkan pembeli", s)
		}
	}
}
