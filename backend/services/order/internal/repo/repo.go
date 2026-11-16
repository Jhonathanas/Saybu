// Package repo mengakses database saybu_orders dengan pgx.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/ids"
	"saybu/pkg/outbox"
	"saybu/services/order/internal/domain"
)

// Repo adalah implementasi akses data pesanan.
type Repo struct {
	pool   *pgxpool.Pool
	outbox *outbox.Store
}

// New membuat Repo.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool, outbox: outbox.NewStore(pool)}
}

// Idempotency adalah baris kunci idempotensi checkout.
type Idempotency struct {
	Key         string
	RequestHash string
	OrderID     string
}

const orderColumns = `
	o.id, o.code, COALESCE(o.user_id::text, ''), o.status, o.fulfillment, o.payment_method,
	o.customer_name, o.customer_phone, o.customer_email, o.note,
	COALESCE(o.zone_id::text, ''), o.zone_name, o.shipping_fee,
	o.address_line, o.address_rt_rw, o.address_kelurahan, o.address_kecamatan,
	o.address_city, o.address_postal_code, o.address_landmark,
	o.subtotal, o.total, o.cancel_reason, o.expires_at, o.created_at, o.updated_at`

func scanOrder(row pgx.Row) (domain.Order, error) {
	var o domain.Order
	err := row.Scan(
		&o.ID, &o.Code, &o.UserID, &o.Status, &o.Fulfillment, &o.PaymentMethod,
		&o.Customer.Name, &o.Customer.Phone, &o.Customer.Email, &o.Customer.Note,
		&o.ZoneID, &o.ZoneName, &o.ShippingFee,
		&o.Address.Line, &o.Address.RTRW, &o.Address.Kelurahan, &o.Address.Kecamatan,
		&o.Address.City, &o.Address.PostalCode, &o.Address.Landmark,
		&o.Subtotal, &o.Total, &o.CancelReason, &o.ExpiresAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return domain.Order{}, err
	}
	return o, nil
}

func loadItems(ctx context.Context, q queryer, o *domain.Order) error {
	rows, err := q.Query(ctx, `
		SELECT product_id::text, name, unit, qty, unit_price, line_total
		FROM order_items
		WHERE order_id = $1
		ORDER BY sort, id
	`, o.ID)
	if err != nil {
		return fmt.Errorf("repo: list order items: %w", err)
	}
	defer rows.Close()
	o.Items = nil
	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Unit, &it.Qty, &it.UnitPrice, &it.LineTotal); err != nil {
			return fmt.Errorf("repo: scan order item: %w", err)
		}
		o.Items = append(o.Items, it)
	}
	return rows.Err()
}

func loadHistory(ctx context.Context, q queryer, o *domain.Order) error {
	rows, err := q.Query(ctx, `
		SELECT status, note, actor, created_at
		FROM order_status_history
		WHERE order_id = $1
		ORDER BY created_at, id
	`, o.ID)
	if err != nil {
		return fmt.Errorf("repo: list history: %w", err)
	}
	defer rows.Close()
	o.History = nil
	for rows.Next() {
		var h domain.StatusEntry
		if err := rows.Scan(&h.Status, &h.Note, &h.Actor, &h.At); err != nil {
			return fmt.Errorf("repo: scan history: %w", err)
		}
		o.History = append(o.History, h)
	}
	return rows.Err()
}

// queryer adalah pgxpool.Pool atau pgx.Tx.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GetIdempotency mengambil kunci idempotensi. found=false bila belum ada.
func (r *Repo) GetIdempotency(ctx context.Context, key string) (Idempotency, bool, error) {
	var it Idempotency
	var orderID *string
	err := r.pool.QueryRow(ctx,
		`SELECT key, request_hash, order_id::text FROM idempotency_keys WHERE key = $1`, key).
		Scan(&it.Key, &it.RequestHash, &orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Idempotency{}, false, nil
	}
	if err != nil {
		return Idempotency{}, false, fmt.Errorf("repo: get idempotency: %w", err)
	}
	if orderID != nil {
		it.OrderID = *orderID
	}
	return it, true, nil
}

// GetOrderByCode mengambil pesanan beserta item & riwayat berdasarkan kode.
func (r *Repo) GetOrderByCode(ctx context.Context, code string) (domain.Order, error) {
	o, err := scanOrder(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders o WHERE o.code = $1`, strings.TrimSpace(code)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("repo: get order by code: %w", err)
	}
	return r.hydrate(ctx, r.pool, o)
}

// GetOrderByID mengambil pesanan berdasarkan id.
func (r *Repo) GetOrderByID(ctx context.Context, id string) (domain.Order, error) {
	o, err := scanOrder(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders o WHERE o.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("repo: get order by id: %w", err)
	}
	return r.hydrate(ctx, r.pool, o)
}

func (r *Repo) hydrate(ctx context.Context, q queryer, o domain.Order) (domain.Order, error) {
	if err := loadItems(ctx, q, &o); err != nil {
		return domain.Order{}, err
	}
	if err := loadHistory(ctx, q, &o); err != nil {
		return domain.Order{}, err
	}
	return o, nil
}

// ListOrdersByUser mengambil pesanan milik user, terbaru lebih dulu.
func (r *Repo) ListOrdersByUser(ctx context.Context, userID string, page, limit int) ([]domain.Order, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM orders WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repo: count orders: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+orderColumns+`
		FROM orders o
		WHERE o.user_id = $1
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, fmt.Errorf("repo: list orders: %w", err)
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repo: scan order: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repo: rows orders: %w", err)
	}
	for i := range out {
		if err := loadItems(ctx, r.pool, &out[i]); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// ListOrders mengambil daftar pesanan untuk admin dengan filter & paginasi.
// Item dimuat agar ringkasan bisa ditampilkan; riwayat tidak dimuat.
func (r *Repo) ListOrders(ctx context.Context, f domain.ListOrdersFilter) ([]domain.Order, int64, error) {
	where, args := orderFilterWhere(f)

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM orders o `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repo: count admin orders: %w", err)
	}

	limIdx := len(args) + 1
	offIdx := len(args) + 2
	queryArgs := append(args, f.Limit, (f.Page-1)*f.Limit)

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM orders o %s
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT $%d OFFSET $%d
	`, orderColumns, where, limIdx, offIdx), queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repo: list admin orders: %w", err)
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repo: scan admin order: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repo: rows admin orders: %w", err)
	}
	for i := range out {
		if err := loadItems(ctx, r.pool, &out[i]); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// orderFilterWhere membangun klausa WHERE untuk filter daftar pesanan admin.
func orderFilterWhere(f domain.ListOrdersFilter) (string, []any) {
	var conds []string
	var args []any
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("o.status = $%d", len(args)))
	}
	if f.DateFrom != "" {
		args = append(args, f.DateFrom)
		conds = append(conds, fmt.Sprintf("(o.created_at AT TIME ZONE 'Asia/Jakarta')::date >= $%d::date", len(args)))
	}
	if f.DateTo != "" {
		args = append(args, f.DateTo)
		conds = append(conds, fmt.Sprintf("(o.created_at AT TIME ZONE 'Asia/Jakarta')::date <= $%d::date", len(args)))
	}
	if f.Query != "" {
		args = append(args, "%"+f.Query+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(o.code ILIKE $%d OR o.customer_name ILIKE $%d OR o.customer_phone ILIKE $%d)", n, n, n))
	}
	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// TodayStats menghitung ringkasan operasional hari ini (Asia/Jakarta).
func (r *Repo) TodayStats(ctx context.Context) (domain.TodayStats, error) {
	var s domain.TodayStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (
				WHERE (o.created_at AT TIME ZONE 'Asia/Jakarta')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date),
			COALESCE(sum(o.total) FILTER (
				WHERE (o.created_at AT TIME ZONE 'Asia/Jakarta')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
				  AND o.status = ANY($1)), 0),
			count(*) FILTER (WHERE o.status = 'WAITING_VERIFICATION')
		FROM orders o
	`, domain.RevenueStatuses).Scan(&s.OrdersCount, &s.RevenuePaid, &s.AwaitingVerification)
	if err != nil {
		return domain.TodayStats{}, fmt.Errorf("repo: today stats: %w", err)
	}
	return s, nil
}

// CreateOrder menyimpan pesanan + item + riwayat + idempotensi + event outbox
// dalam satu transaksi. Kode pesanan dibuat dari counter harian.
func (r *Repo) CreateOrder(ctx context.Context, o domain.Order, idem Idempotency, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, fmt.Errorf("repo: begin create order: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Nomor urut harian (concurrency-safe).
	var day time.Time
	var seq int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO order_code_counters (day, last_seq)
		VALUES ((now() AT TIME ZONE 'Asia/Jakarta')::date, 1)
		ON CONFLICT (day) DO UPDATE SET last_seq = order_code_counters.last_seq + 1
		RETURNING day, last_seq
	`).Scan(&day, &seq); err != nil {
		return domain.Order{}, fmt.Errorf("repo: next order code: %w", err)
	}
	o.Code = ids.OrderCode(day, seq)
	o.Status = domain.StatusPendingPayment

	if _, err := tx.Exec(ctx, `
		INSERT INTO orders (id, code, user_id, status, fulfillment, payment_method,
			customer_name, customer_phone, customer_email, note,
			zone_id, zone_name, shipping_fee,
			address_line, address_rt_rw, address_kelurahan, address_kecamatan,
			address_city, address_postal_code, address_landmark,
			subtotal, total, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
	`, o.ID, o.Code, nullUUID(o.UserID), o.Status, o.Fulfillment, o.PaymentMethod,
		o.Customer.Name, o.Customer.Phone, o.Customer.Email, o.Customer.Note,
		nullUUID(o.ZoneID), o.ZoneName, o.ShippingFee,
		o.Address.Line, o.Address.RTRW, o.Address.Kelurahan, o.Address.Kecamatan,
		o.Address.City, o.Address.PostalCode, o.Address.Landmark,
		o.Subtotal, o.Total, o.ExpiresAt); err != nil {
		return domain.Order{}, fmt.Errorf("repo: insert order: %w", err)
	}

	for i, it := range o.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, name, unit, qty, unit_price, line_total, sort)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8)
		`, o.ID, it.ProductID, it.Name, it.Unit, it.Qty, it.UnitPrice, it.LineTotal, i); err != nil {
			return domain.Order{}, fmt.Errorf("repo: insert order item: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, status, note, actor)
		VALUES (gen_random_uuid(), $1, $2, '', 'system')
	`, o.ID, o.Status); err != nil {
		return domain.Order{}, fmt.Errorf("repo: insert initial history: %w", err)
	}

	if idem.Key != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO idempotency_keys (key, request_hash, order_id)
			VALUES ($1, $2, $3)
		`, idem.Key, idem.RequestHash, o.ID); err != nil {
			if isUniqueViolation(err) {
				return domain.Order{}, domain.ErrIdempotencyExists
			}
			return domain.Order{}, fmt.Errorf("repo: insert idempotency: %w", err)
		}
	}

	o.CreatedAt = time.Now().UTC()
	o.UpdatedAt = o.CreatedAt
	if events != nil {
		recs, err := events(o)
		if err != nil {
			return domain.Order{}, err
		}
		if err := insertEvents(ctx, r.outbox, tx, recs); err != nil {
			return domain.Order{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, fmt.Errorf("repo: commit create order: %w", err)
	}
	return r.GetOrderByID(ctx, o.ID)
}

// Transition memindahkan status pesanan secara atomik (WHERE status = from).
func (r *Repo) Transition(ctx context.Context, id, from, to, note, actor string, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, fmt.Errorf("repo: begin transition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE orders SET
			status = $3,
			cancel_reason = CASE WHEN $3 = 'CANCELLED' THEN $4 ELSE cancel_reason END,
			updated_at = now()
		WHERE id = $1 AND status = $2
	`, id, from, to, note)
	if err != nil {
		return domain.Order{}, fmt.Errorf("repo: update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.ErrInvalidTransition
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, status, note, actor)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
	`, id, to, note, actor); err != nil {
		return domain.Order{}, fmt.Errorf("repo: insert history: %w", err)
	}

	o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders o WHERE o.id = $1`, id))
	if err != nil {
		return domain.Order{}, fmt.Errorf("repo: reload order: %w", err)
	}
	if err := loadItems(ctx, tx, &o); err != nil {
		return domain.Order{}, err
	}

	if events != nil {
		recs, err := events(o)
		if err != nil {
			return domain.Order{}, err
		}
		if err := insertEvents(ctx, r.outbox, tx, recs); err != nil {
			return domain.Order{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, fmt.Errorf("repo: commit transition: %w", err)
	}
	return r.GetOrderByID(ctx, id)
}

// TransitionFromPayment memindahkan status pesanan akibat event pembayaran
// secara idempotent. Event dicatat di processed_events dalam transaksi yang
// sama; bila event sudah pernah diproses, applied=false tanpa efek.
// Bila pesanan tidak berada pada status `from`, event tetap ditandai diproses
// (retry tidak akan menolong) dan applied=false.
func (r *Repo) TransitionFromPayment(ctx context.Context, eventID, eventType, orderID, from, to, note, actor string, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("repo: begin payment transition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id, event_type) VALUES ($1, $2)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID, eventType)
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("repo: insert processed event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, false, nil
	}

	tag, err = tx.Exec(ctx, `
		UPDATE orders SET status = $3, updated_at = now()
		WHERE id = $1 AND status = $2
	`, orderID, from, to)
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("repo: update status from payment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Pesanan tidak pada status yang diharapkan; tetap tandai event selesai.
		if err := tx.Commit(ctx); err != nil {
			return domain.Order{}, false, fmt.Errorf("repo: commit payment transition: %w", err)
		}
		return domain.Order{}, false, nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, status, note, actor)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
	`, orderID, to, note, actor); err != nil {
		return domain.Order{}, false, fmt.Errorf("repo: insert history: %w", err)
	}

	o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders o WHERE o.id = $1`, orderID))
	if err != nil {
		return domain.Order{}, false, fmt.Errorf("repo: reload order: %w", err)
	}
	if err := loadItems(ctx, tx, &o); err != nil {
		return domain.Order{}, false, err
	}

	if events != nil {
		recs, err := events(o)
		if err != nil {
			return domain.Order{}, false, err
		}
		if err := insertEvents(ctx, r.outbox, tx, recs); err != nil {
			return domain.Order{}, false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Order{}, false, fmt.Errorf("repo: commit payment transition: %w", err)
	}
	return o, true, nil
}

// ExpireDue menandai pesanan PENDING_PAYMENT yang melewati expires_at sebagai
// EXPIRED, menulis riwayat + event, dan mengembalikan pesanan yang kedaluwarsa.
// Memakai FOR UPDATE SKIP LOCKED agar aman dijalankan paralel.
func (r *Repo) ExpireDue(ctx context.Context, limit int, events func(domain.Order) ([]outbox.Record, error)) ([]domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repo: begin expire: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id FROM orders
		WHERE status = 'PENDING_PAYMENT' AND expires_at <= now()
		ORDER BY expires_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("repo: select due orders: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("repo: scan due order: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: rows due orders: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	var expired []domain.Order
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `
			UPDATE orders SET status = 'EXPIRED', updated_at = now() WHERE id = $1
		`, id); err != nil {
			return nil, fmt.Errorf("repo: mark expired: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_status_history (id, order_id, status, note, actor)
			VALUES (gen_random_uuid(), $1, 'EXPIRED', 'Kedaluwarsa otomatis', 'system')
		`, id); err != nil {
			return nil, fmt.Errorf("repo: insert expired history: %w", err)
		}
		o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders o WHERE o.id = $1`, id))
		if err != nil {
			return nil, fmt.Errorf("repo: reload expired order: %w", err)
		}
		if err := loadItems(ctx, tx, &o); err != nil {
			return nil, err
		}
		if events != nil {
			recs, err := events(o)
			if err != nil {
				return nil, err
			}
			if err := insertEvents(ctx, r.outbox, tx, recs); err != nil {
				return nil, err
			}
		}
		expired = append(expired, o)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repo: commit expire: %w", err)
	}
	return expired, nil
}

func insertEvents(ctx context.Context, store *outbox.Store, tx pgx.Tx, events []outbox.Record) error {
	for _, ev := range events {
		if err := store.Insert(ctx, tx, ev); err != nil {
			return err
		}
	}
	return nil
}

func nullUUID(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
