package domain

// transitions mendefinisikan transisi status pesanan yang sah.
//
// Sumber: docs/ARSITEKTUR.md §7.2.
var transitions = map[string][]string{
	StatusPendingPayment:      {StatusWaitingVerification, StatusPaid, StatusCancelled, StatusExpired},
	StatusWaitingVerification: {StatusPaid, StatusPendingPayment, StatusCancelled},
	StatusPaid:                {StatusPreparing, StatusCancelled},
	StatusPreparing:           {StatusShipped, StatusReadyPickup, StatusCancelled},
	StatusShipped:             {StatusCompleted},
	StatusReadyPickup:         {StatusCompleted},
	StatusCompleted:           {},
	StatusCancelled:           {},
	StatusExpired:             {},
}

// AllStatuses adalah daftar seluruh status yang sah.
var AllStatuses = []string{
	StatusPendingPayment, StatusWaitingVerification, StatusPaid, StatusPreparing,
	StatusShipped, StatusReadyPickup, StatusCompleted, StatusCancelled, StatusExpired,
}

// IsValidStatus melaporkan apakah status dikenal.
func IsValidStatus(status string) bool {
	_, ok := transitions[status]
	return ok
}

// CanTransition melaporkan apakah perpindahan status from -> to sah.
func CanTransition(from, to string) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// cancellableByCustomer adalah status yang boleh dibatalkan pembeli (guest/akun).
func cancellableByCustomer(status string) bool {
	switch status {
	case StatusPendingPayment, StatusWaitingVerification, StatusPaid:
		return true
	default:
		return false
	}
}

// IsTerminal melaporkan apakah status tidak punya transisi keluar.
func IsTerminal(status string) bool {
	return len(transitions[status]) == 0
}
