// Package repo mengakses database saybu_payments dengan pgx.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
	"saybu/pkg/outbox"
	"saybu/services/payment/internal/domain"
)

// Repo adalah implementasi akses data pembayaran.
type Repo struct {
	pool   *pgxpool.Pool
	outbox *outbox.Store
}

// New membuat Repo.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool, outbox: outbox.NewStore(pool)}
}

const paymentColumns = `
	p.id, p.order_id::text, p.order_code, COALESCE(p.user_id::text, ''), p.method, p.status, p.amount,
	p.customer_name, p.customer_phone,
	p.proof_url, p.proof_uploaded_at, p.verified_at, p.verified_by, p.note,
	p.created_at, p.updated_at`

func scanPayment(row pgx.Row) (domain.Payment, error) {
	var p domain.Payment
	err := row.Scan(
		&p.ID, &p.OrderID, &p.OrderCode, &p.UserID, &p.Method, &p.Status, &p.Amount,
		&p.CustomerName, &p.CustomerPhone,
		&p.ProofURL, &p.ProofUploadedAt, &p.VerifiedAt, &p.VerifiedBy, &p.Note,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return domain.Payment{}, err
	}
	return p, nil
}

// CreateInput adalah data pembayaran baru dari event order.created.
type CreateInput struct {
	EventID       string
	OrderID       string
	OrderCode     string
	UserID        string
	Method        string
	Amount        int64
	CustomerName  string
	CustomerPhone string
}

// CreateFromOrder membuat pembayaran dari event order.created secara idempotent.
// Mengembalikan created=false bila event sudah pernah diproses.
func (r *Repo) CreateFromOrder(ctx context.Context, in CreateInput) (domain.Payment, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, false, fmt.Errorf("repo: begin create payment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Idempotensi consumer: event yang sama diproses sekali.
	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id, event_type) VALUES ($1, 'order.created')
		ON CONFLICT (event_id) DO NOTHING
	`, in.EventID)
	if err != nil {
		return domain.Payment{}, false, fmt.Errorf("repo: insert processed event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		p, err := r.getByOrderID(ctx, tx, in.OrderID)
		return p, false, err
	}

	p := domain.Payment{
		ID:            ids.New(),
		OrderID:       in.OrderID,
		OrderCode:     in.OrderCode,
		UserID:        in.UserID,
		Method:        in.Method,
		Status:        domain.InitialStatus(in.Method),
		Amount:        in.Amount,
		CustomerName:  in.CustomerName,
		CustomerPhone: in.CustomerPhone,
	}
	tag, err = tx.Exec(ctx, `
		INSERT INTO payments (id, order_id, order_code, user_id, method, status, amount,
			customer_name, customer_phone)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (order_id) DO NOTHING
	`, p.ID, p.OrderID, p.OrderCode, nullUUID(p.UserID), p.Method, p.Status, p.Amount,
		p.CustomerName, p.CustomerPhone)
	if err != nil {
		return domain.Payment{}, false, fmt.Errorf("repo: insert payment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		existing, err := r.getByOrderID(ctx, tx, in.OrderID)
		return existing, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Payment{}, false, fmt.Errorf("repo: commit create payment: %w", err)
	}
	created, err := r.GetByOrderID(ctx, in.OrderID)
	if err != nil {
		return domain.Payment{}, false, err
	}
	return created, true, nil
}

// GetByOrderCode mengambil pembayaran berdasarkan kode pesanan.
func (r *Repo) GetByOrderCode(ctx context.Context, code string) (domain.Payment, error) {
	p, err := scanPayment(r.pool.QueryRow(ctx,
		`SELECT `+paymentColumns+` FROM payments p WHERE p.order_code = $1`, strings.TrimSpace(code)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: get payment by code: %w", err)
	}
	return p, nil
}

// GetByOrderID mengambil pembayaran berdasarkan id pesanan.
func (r *Repo) GetByOrderID(ctx context.Context, orderID string) (domain.Payment, error) {
	return r.getByOrderID(ctx, r.pool, orderID)
}

// GetByID mengambil pembayaran berdasarkan id.
func (r *Repo) GetByID(ctx context.Context, id string) (domain.Payment, error) {
	p, err := scanPayment(r.pool.QueryRow(ctx,
		`SELECT `+paymentColumns+` FROM payments p WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: get payment by id: %w", err)
	}
	return p, nil
}

// List mengambil daftar pembayaran dengan filter status opsional.
func (r *Repo) List(ctx context.Context, status string, page, limit int) ([]domain.Payment, int64, error) {
	status = strings.TrimSpace(status)
	where := ""
	args := []any{}
	if status != "" {
		where = " WHERE status = $1"
		args = append(args, status)
	}

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM payments`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repo: count payments: %w", err)
	}

	args = append(args, limit, (page-1)*limit)
	rows, err := r.pool.Query(ctx, `
		SELECT `+paymentColumns+`
		FROM payments p`+where+`
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repo: list payments: %w", err)
	}
	defer rows.Close()

	var out []domain.Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repo: scan payment: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repo: rows payments: %w", err)
	}
	return out, total, nil
}

// AttachProof menyimpan bukti transfer baru dan mengubah status menjadi
// WAITING_VERIFICATION. Hanya berlaku pada status UNPAID/REJECTED.
func (r *Repo) AttachProof(ctx context.Context, paymentID string, proof domain.Proof, events func(domain.Payment) ([]outbox.Record, error)) (domain.Payment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: begin attach proof: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_proofs (id, payment_id, url, size_bytes, content_type, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, proof.ID, paymentID, proof.URL, proof.SizeBytes, proof.ContentType, proof.UploadedBy); err != nil {
		return domain.Payment{}, fmt.Errorf("repo: insert proof: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE payments SET
			proof_url = $2,
			proof_uploaded_at = now(),
			status = 'WAITING_VERIFICATION',
			note = '',
			updated_at = now()
		WHERE id = $1 AND status IN ('UNPAID', 'REJECTED')
	`, paymentID, proof.URL)
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: update payment proof: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Payment{}, domain.ErrProofNotAllowed
	}

	p, err := scanPayment(tx.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payments p WHERE p.id = $1`, paymentID))
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: reload payment: %w", err)
	}
	if events != nil {
		recs, err := events(p)
		if err != nil {
			return domain.Payment{}, err
		}
		if err := insertEvents(ctx, r.outbox, tx, recs); err != nil {
			return domain.Payment{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Payment{}, fmt.Errorf("repo: commit attach proof: %w", err)
	}
	return p, nil
}

// Verify mengubah status pembayaran menjadi PAID atau REJECTED.
// Hanya berlaku bila pembayaran menunggu verifikasi.
func (r *Repo) Verify(ctx context.Context, paymentID, toStatus, note, verifiedBy string, events func(domain.Payment) ([]outbox.Record, error)) (domain.Payment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: begin verify: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE payments SET
			status = $2,
			note = $3,
			verified_at = now(),
			verified_by = $4,
			updated_at = now()
		WHERE id = $1 AND status = 'WAITING_VERIFICATION'
	`, paymentID, toStatus, note, verifiedBy)
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: update verify: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Payment{}, domain.ErrVerifyNotAllowed
	}

	p, err := scanPayment(tx.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payments p WHERE p.id = $1`, paymentID))
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: reload payment: %w", err)
	}
	if events != nil {
		recs, err := events(p)
		if err != nil {
			return domain.Payment{}, err
		}
		if err := insertEvents(ctx, r.outbox, tx, recs); err != nil {
			return domain.Payment{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Payment{}, fmt.Errorf("repo: commit verify: %w", err)
	}
	return p, nil
}

// CancelFromOrderEvent menandai pembayaran CANCELLED saat pesanan batal/kedaluwarsa,
// idempotent terhadap event_id. Mengembalikan processed=false bila sudah diproses.
func (r *Repo) CancelFromOrderEvent(ctx context.Context, eventID, eventType, orderID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("repo: begin cancel payment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id, event_type) VALUES ($1, $2)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID, eventType)
	if err != nil {
		return false, fmt.Errorf("repo: insert processed event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE payments SET status = 'CANCELLED', updated_at = now()
		WHERE order_id = $1 AND status NOT IN ('PAID', 'CANCELLED', 'COD_PAID')
	`, orderID); err != nil {
		return false, fmt.Errorf("repo: cancel payment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("repo: commit cancel payment: %w", err)
	}
	return true, nil
}

func (r *Repo) getByOrderID(ctx context.Context, q queryer, orderID string) (domain.Payment, error) {
	p, err := scanPayment(q.QueryRow(ctx,
		`SELECT `+paymentColumns+` FROM payments p WHERE p.order_id = $1`, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("repo: get payment by order id: %w", err)
	}
	return p, nil
}

// queryer adalah pgxpool.Pool atau pgx.Tx.
type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertEvents(ctx context.Context, store *outbox.Store, tx pgx.Tx, events []outbox.Record) error {
	for _, ev := range events {
		if err := store.Insert(ctx, tx, ev); err != nil {
			return err
		}
	}
	return nil
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func nullUUID(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
