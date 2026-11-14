// Package repo mengakses database saybu_catalog dengan pgx.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"saybu/pkg/outbox"
	"saybu/services/catalog/internal/domain"
)

// Repo adalah implementasi akses data katalog.
type Repo struct {
	pool   *pgxpool.Pool
	outbox *outbox.Store
}

// New membuat Repo.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool, outbox: outbox.NewStore(pool)}
}

const productColumns = `
	p.id, p.name, p.slug, p.description, p.unit, p.price, p.stock,
	p.min_qty, p.step_qty, p.low_stock_threshold, p.is_active,
	p.created_at, p.updated_at,
	c.id, c.name, c.slug, c.sort, c.is_active`

func scanProduct(row pgx.Row) (domain.Product, error) {
	var p domain.Product
	var catID, catName, catSlug *string
	var catSort *int32
	var catActive *bool
	err := row.Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.Unit, &p.Price, &p.Stock,
		&p.MinQty, &p.StepQty, &p.LowStockThreshold, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt,
		&catID, &catName, &catSlug, &catSort, &catActive,
	)
	if err != nil {
		return domain.Product{}, err
	}
	if catID != nil {
		c := &domain.Category{ID: *catID, Name: deref(catName), Slug: deref(catSlug)}
		if catSort != nil {
			c.Sort = *catSort
		}
		if catActive != nil {
			c.IsActive = *catActive
		}
		p.Category = c
	}
	return p, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ProductFilter adalah parameter pencarian produk.
type ProductFilter = domain.ProductFilter

// orderClause mengembalikan ORDER BY yang aman (whitelist).
func orderClause(sort string) string {
	switch sort {
	case "price_asc":
		return "ORDER BY p.price ASC, p.id ASC"
	case "price_desc":
		return "ORDER BY p.price DESC, p.id ASC"
	default:
		return "ORDER BY p.created_at DESC, p.id ASC"
	}
}

// ListProducts mengambil daftar produk beserta total.
func (r *Repo) ListProducts(ctx context.Context, f ProductFilter) ([]domain.Product, int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE ($1 = '' OR c.slug = $1)
		  AND ($2 = '' OR p.name ILIKE '%' || $2 || '%')
		  AND ($3 OR p.is_active)
	`, f.CategorySlug, strings.TrimSpace(f.Query), f.IncludeInactive).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repo: count products: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+productColumns+`
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE ($1 = '' OR c.slug = $1)
		  AND ($2 = '' OR p.name ILIKE '%' || $2 || '%')
		  AND ($3 OR p.is_active)
		`+orderClause(f.Sort)+`
		LIMIT $4 OFFSET $5
	`, f.CategorySlug, strings.TrimSpace(f.Query), f.IncludeInactive, f.Limit, (f.Page-1)*f.Limit)
	if err != nil {
		return nil, 0, fmt.Errorf("repo: list products: %w", err)
	}
	defer rows.Close()

	products, err := collectProducts(rows)
	if err != nil {
		return nil, 0, err
	}
	if err := r.attachImages(ctx, products); err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func collectProducts(rows pgx.Rows) ([]domain.Product, error) {
	var out []domain.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, fmt.Errorf("repo: scan product: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: rows products: %w", err)
	}
	return out, nil
}

// attachImages mengisi daftar gambar untuk kumpulan produk (menghindari N+1).
func (r *Repo) attachImages(ctx context.Context, products []domain.Product) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]string, 0, len(products))
	index := make(map[string]int, len(products))
	for i, p := range products {
		ids = append(ids, p.ID)
		index[p.ID] = i
	}
	rows, err := r.pool.Query(ctx, `
		SELECT product_id, url
		FROM product_images
		WHERE product_id = ANY($1)
		ORDER BY sort, created_at
	`, ids)
	if err != nil {
		return fmt.Errorf("repo: list images: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pid, url string
		if err := rows.Scan(&pid, &url); err != nil {
			return fmt.Errorf("repo: scan image: %w", err)
		}
		if i, ok := index[pid]; ok {
			products[i].Images = append(products[i].Images, url)
		}
	}
	return rows.Err()
}

// GetProductByID mengambil produk berdasarkan id.
func (r *Repo) GetProductByID(ctx context.Context, id string, includeInactive bool) (domain.Product, error) {
	p, err := scanProduct(r.pool.QueryRow(ctx, `
		SELECT `+productColumns+`
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.id = $1 AND ($2 OR p.is_active)
	`, id, includeInactive))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Product{}, domain.ErrProductNotFound
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("repo: get product by id: %w", err)
	}
	return r.reloadOne(ctx, p.ID, &p)
}

// reloadOne mengambil ulang gambar untuk satu produk.
func (r *Repo) reloadOne(ctx context.Context, id string, p *domain.Product) (domain.Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT url FROM product_images WHERE product_id = $1 ORDER BY sort, created_at`, id)
	if err != nil {
		return domain.Product{}, fmt.Errorf("repo: get images: %w", err)
	}
	defer rows.Close()
	p.Images = nil
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return domain.Product{}, fmt.Errorf("repo: scan image: %w", err)
		}
		p.Images = append(p.Images, url)
	}
	return *p, rows.Err()
}

// GetProductBySlug mengambil produk berdasarkan slug.
func (r *Repo) GetProductBySlug(ctx context.Context, slug string, includeInactive bool) (domain.Product, error) {
	p, err := scanProduct(r.pool.QueryRow(ctx, `
		SELECT `+productColumns+`
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.slug = $1 AND ($2 OR p.is_active)
	`, slug, includeInactive))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Product{}, domain.ErrProductNotFound
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("repo: get product by slug: %w", err)
	}
	return r.reloadOne(ctx, p.ID, &p)
}

// ListCategories mengambil daftar kategori.
func (r *Repo) ListCategories(ctx context.Context, includeInactive bool) ([]domain.Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, slug, sort, is_active
		FROM categories
		WHERE $1 OR is_active
		ORDER BY sort, name
	`, includeInactive)
	if err != nil {
		return nil, fmt.Errorf("repo: list categories: %w", err)
	}
	defer rows.Close()

	var out []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Sort, &c.IsActive); err != nil {
			return nil, fmt.Errorf("repo: scan category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCategory mengambil kategori berdasarkan id.
func (r *Repo) GetCategory(ctx context.Context, id string) (domain.Category, error) {
	var c domain.Category
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, sort, is_active FROM categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Slug, &c.Sort, &c.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Category{}, domain.ErrCategoryNotFound
	}
	if err != nil {
		return domain.Category{}, fmt.Errorf("repo: get category: %w", err)
	}
	return c, nil
}

// CreateProduct menyimpan produk baru beserta event outbox dalam satu transaksi.
func (r *Repo) CreateProduct(ctx context.Context, p domain.Product, events []outbox.Record) (domain.Product, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Product{}, fmt.Errorf("repo: begin create product: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO products (id, category_id, name, slug, description, unit, price, stock,
			min_qty, step_qty, low_stock_threshold, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, p.ID, nullableUUID(p.Category), p.Name, p.Slug, p.Description, p.Unit, p.Price, p.Stock,
		p.MinQty, p.StepQty, p.LowStockThreshold, p.IsActive); err != nil {
		return domain.Product{}, mapWriteErr("create product", err)
	}

	if err := insertImages(ctx, tx, p.ID, p.Images); err != nil {
		return domain.Product{}, err
	}
	if err := insertEvents(ctx, r.outbox, tx, events); err != nil {
		return domain.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Product{}, fmt.Errorf("repo: commit create product: %w", err)
	}
	return r.GetProductByID(ctx, p.ID, true)
}

// UpdateProduct memperbarui produk (mengganti daftar gambar) + event outbox.
func (r *Repo) UpdateProduct(ctx context.Context, p domain.Product, events []outbox.Record) (domain.Product, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Product{}, fmt.Errorf("repo: begin update product: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE products SET
			category_id = $2, name = $3, slug = $4, description = $5, unit = $6,
			price = $7, stock = $8, min_qty = $9, step_qty = $10,
			low_stock_threshold = $11, is_active = $12, updated_at = now()
		WHERE id = $1
	`, p.ID, nullableUUID(p.Category), p.Name, p.Slug, p.Description, p.Unit,
		p.Price, p.Stock, p.MinQty, p.StepQty, p.LowStockThreshold, p.IsActive)
	if err != nil {
		return domain.Product{}, mapWriteErr("update product", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Product{}, domain.ErrProductNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM product_images WHERE product_id = $1`, p.ID); err != nil {
		return domain.Product{}, fmt.Errorf("repo: clear images: %w", err)
	}
	if err := insertImages(ctx, tx, p.ID, p.Images); err != nil {
		return domain.Product{}, err
	}
	if err := insertEvents(ctx, r.outbox, tx, events); err != nil {
		return domain.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Product{}, fmt.Errorf("repo: commit update product: %w", err)
	}
	return r.GetProductByID(ctx, p.ID, true)
}

// SetStock mengubah stok produk dan menulis event outbox.
func (r *Repo) SetStock(ctx context.Context, id string, newStock int64, events []outbox.Record) (domain.Product, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Product{}, fmt.Errorf("repo: begin set stock: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE products SET stock = $2, updated_at = now() WHERE id = $1`, id, newStock)
	if err != nil {
		return domain.Product{}, mapWriteErr("set stock", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Product{}, domain.ErrProductNotFound
	}
	if err := insertEvents(ctx, r.outbox, tx, events); err != nil {
		return domain.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Product{}, fmt.Errorf("repo: commit set stock: %w", err)
	}
	return r.GetProductByID(ctx, id, true)
}

// SetActive mengubah status aktif produk (soft delete) + event outbox.
func (r *Repo) SetActive(ctx context.Context, id string, active bool, events []outbox.Record) (domain.Product, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Product{}, fmt.Errorf("repo: begin set active: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE products SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	if err != nil {
		return domain.Product{}, mapWriteErr("set active", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Product{}, domain.ErrProductNotFound
	}
	if err := insertEvents(ctx, r.outbox, tx, events); err != nil {
		return domain.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Product{}, fmt.Errorf("repo: commit set active: %w", err)
	}
	return r.GetProductByID(ctx, id, true)
}

// CheckStock melaporkan ketersediaan stok untuk kumpulan item.
func (r *Repo) CheckStock(ctx context.Context, items []domain.StockItem) ([]domain.StockStatus, error) {
	out := make([]domain.StockStatus, 0, len(items))
	for _, it := range items {
		var name string
		var stock int64
		err := r.pool.QueryRow(ctx,
			`SELECT name, stock FROM products WHERE id = $1`, it.ProductID).Scan(&name, &stock)
		if errors.Is(err, pgx.ErrNoRows) {
			out = append(out, domain.StockStatus{ProductID: it.ProductID, Requested: it.Qty})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("repo: check stock: %w", err)
		}
		out = append(out, domain.StockStatus{
			ProductID: it.ProductID,
			Name:      name,
			Requested: it.Qty,
			Available: stock,
			OK:        stock >= it.Qty,
		})
	}
	return out, nil
}

// ReserveStock mengurangi stok dan mencatat reservasi untuk sebuah order.
// Bila ada item yang kurang, seluruh transaksi dibatalkan dan item yang gagal dikembalikan.
func (r *Repo) ReserveStock(ctx context.Context, orderID string, items []domain.StockItem) ([]domain.StockStatus, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repo: begin reserve: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var insufficient []domain.StockStatus
	for _, it := range items {
		var name string
		var stock int64
		err := tx.QueryRow(ctx,
			`SELECT name, stock FROM products WHERE id = $1 AND is_active FOR UPDATE`,
			it.ProductID).Scan(&name, &stock)
		if errors.Is(err, pgx.ErrNoRows) {
			insufficient = append(insufficient, domain.StockStatus{ProductID: it.ProductID, Requested: it.Qty})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("repo: lock product: %w", err)
		}
		if stock < it.Qty {
			insufficient = append(insufficient, domain.StockStatus{
				ProductID: it.ProductID, Name: name, Requested: it.Qty, Available: stock,
			})
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE products SET stock = stock - $2, updated_at = now() WHERE id = $1
		`, it.ProductID, it.Qty); err != nil {
			return nil, fmt.Errorf("repo: reserve update: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_reservations (id, order_id, product_id, qty, status)
			VALUES (gen_random_uuid(), $1, $2, $3, 'RESERVED')
		`, orderID, it.ProductID, it.Qty); err != nil {
			return nil, fmt.Errorf("repo: insert reservation: %w", err)
		}
	}

	if len(insufficient) > 0 {
		return insufficient, nil // rollback via defer
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repo: commit reserve: %w", err)
	}
	return nil, nil
}

// ReleaseStock mengembalikan stok untuk reservasi order yang masih RESERVED.
func (r *Repo) ReleaseStock(ctx context.Context, orderID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repo: begin release: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := releaseStockTx(ctx, tx, orderID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repo: commit release: %w", err)
	}
	return nil
}

// ReleaseStockOnce melepas stok untuk sebuah event, idempotent terhadap event_id.
// Mengembalikan processed=false bila event sudah pernah diproses (tidak ada efek).
func (r *Repo) ReleaseStockOnce(ctx context.Context, eventID, orderID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("repo: begin release once: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id) VALUES ($1)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID)
	if err != nil {
		return false, fmt.Errorf("repo: insert processed event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil // sudah diproses
	}

	if err := releaseStockTx(ctx, tx, orderID); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("repo: commit release once: %w", err)
	}
	return true, nil
}

// releaseStockTx melepas reservasi RESERVED milik order dalam transaksi yang ada.
func releaseStockTx(ctx context.Context, tx pgx.Tx, orderID string) error {
	rows, err := tx.Query(ctx, `
		SELECT id, product_id, qty
		FROM stock_reservations
		WHERE order_id = $1 AND status = 'RESERVED'
		FOR UPDATE
	`, orderID)
	if err != nil {
		return fmt.Errorf("repo: select reservations: %w", err)
	}
	type res struct {
		id, productID string
		qty           int64
	}
	var list []res
	for rows.Next() {
		var x res
		if err := rows.Scan(&x.id, &x.productID, &x.qty); err != nil {
			rows.Close()
			return fmt.Errorf("repo: scan reservation: %w", err)
		}
		list = append(list, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("repo: rows reservations: %w", err)
	}

	for _, x := range list {
		if _, err := tx.Exec(ctx,
			`UPDATE products SET stock = stock + $2, updated_at = now() WHERE id = $1`,
			x.productID, x.qty); err != nil {
			return fmt.Errorf("repo: release update: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE stock_reservations SET status = 'RELEASED', updated_at = now() WHERE id = $1`,
			x.id); err != nil {
			return fmt.Errorf("repo: mark released: %w", err)
		}
	}
	return nil
}

// ConfirmStock mengubah reservasi menjadi pemakaian final (setelah pembayaran).
func (r *Repo) ConfirmStock(ctx context.Context, orderID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE stock_reservations SET status = 'CONFIRMED', updated_at = now()
		WHERE order_id = $1 AND status = 'RESERVED'
	`, orderID)
	if err != nil {
		return fmt.Errorf("repo: confirm stock: %w", err)
	}
	return nil
}

func insertImages(ctx context.Context, tx pgx.Tx, productID string, urls []string) error {
	for i, url := range urls {
		if strings.TrimSpace(url) == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_images (id, product_id, url, sort)
			VALUES (gen_random_uuid(), $1, $2, $3)
		`, productID, url, i); err != nil {
			return fmt.Errorf("repo: insert image: %w", err)
		}
	}
	return nil
}

func insertEvents(ctx context.Context, store *outbox.Store, tx pgx.Tx, events []outbox.Record) error {
	for _, ev := range events {
		if err := store.Insert(ctx, tx, ev); err != nil {
			return err
		}
	}
	return nil
}

func nullableUUID(c *domain.Category) any {
	if c == nil || c.ID == "" {
		return nil
	}
	return c.ID
}

func mapWriteErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrSlugExists
		case "23503":
			return domain.ErrCategoryNotFound
		case "23514":
			if strings.Contains(pgErr.ConstraintName, "stock") {
				return domain.ErrStockNegative
			}
		}
	}
	return fmt.Errorf("repo: %s: %w", op, err)
}
