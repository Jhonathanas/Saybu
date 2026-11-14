// Command seed mengisi data contoh Saybu secara idempoten.
//
// Aman dijalankan berulang: baris dicari berdasarkan kunci alami (slug
// kategori/produk, nomor HP admin, nama zona) lalu diperbarui, bukan diduplikasi.
//
//	go run ./cmd/seed
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"

	"saybu/pkg/config"
	"saybu/pkg/ids"
	logpkg "saybu/pkg/logger"
)

type categorySeed struct {
	name string
	slug string
	sort int
}

type productSeed struct {
	categorySlug string
	name         string
	slug         string
	description  string
	unit         string
	price        int64
	stock        int64
	minQty       int64
	stepQty      int64
}

type zoneSeed struct {
	name            string
	areas           []string
	fee             int64
	freeMinSubtotal int64
}

func main() {
	config.LoadDotEnv()
	log := logpkg.New("seed", config.Get("LOG_LEVEL", "info"))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	catalogPool, err := pgxpool.New(ctx, config.MustGet("CATALOG_DB_DSN"))
	if err != nil {
		log.Error("koneksi postgres katalog gagal", "error", err)
		os.Exit(1)
	}
	defer catalogPool.Close()

	authPool, err := pgxpool.New(ctx, config.MustGet("AUTH_DB_DSN"))
	if err != nil {
		log.Error("koneksi postgres auth gagal", "error", err)
		os.Exit(1)
	}
	defer authPool.Close()

	shippingPool, err := pgxpool.New(ctx, config.MustGet("SHIPPING_DB_DSN"))
	if err != nil {
		log.Error("koneksi postgres shipping gagal", "error", err)
		os.Exit(1)
	}
	defer shippingPool.Close()

	cats, err := seedCategories(ctx, catalogPool)
	if err != nil {
		log.Error("seed kategori gagal", "error", err)
		os.Exit(1)
	}
	n, err := seedProducts(ctx, catalogPool, cats)
	if err != nil {
		log.Error("seed produk gagal", "error", err)
		os.Exit(1)
	}
	admin, err := seedAdmin(ctx, authPool)
	if err != nil {
		log.Error("seed admin gagal", "error", err)
		os.Exit(1)
	}
	zones, err := seedZones(ctx, shippingPool)
	if err != nil {
		log.Error("seed zona ongkir gagal", "error", err)
		os.Exit(1)
	}

	log.Info("seed selesai", "kategori", len(cats), "produk", n, "admin", admin, "zona", zones)
}

func categories() []categorySeed {
	return []categorySeed{
		{name: "Buah", slug: "buah", sort: 1},
		{name: "Sayur", slug: "sayur", sort: 2},
	}
}

func products() []productSeed {
	return []productSeed{
		{"buah", "Pisang Cavendish", "pisang-cavendish", "Pisang manis pulen, cocok untuk sarapan.", "kg", 18000, 25000, 250, 250},
		{"buah", "Jeruk Baby Pacitan", "jeruk-baby-pacitan", "Jeruk baby manis segar dari Pacitan.", "kg", 25000, 20000, 250, 250},
		{"buah", "Apel Fuji", "apel-fuji", "Apel Fuji renyah dengan rasa manis seimbang.", "kg", 32000, 15000, 250, 250},
		{"buah", "Semangka Merah", "semangka-merah", "Semangka merah tanpa biji, manis dan berair.", "buah", 35000, 20, 1, 1},
		{"buah", "Mangga Harum Manis", "mangga-harum-manis", "Mangga harum manis matang pohon.", "kg", 28000, 12000, 250, 250},
		{"buah", "Alpukat Mentega", "alpukat-mentega", "Alpukat mentega daging tebal dan pulen.", "buah", 12000, 30, 1, 1},
		{"sayur", "Bayam Segar", "bayam-segar", "Bayam hijau segar hasil panen pagi.", "ikat", 5000, 40, 1, 1},
		{"sayur", "Kangkung", "kangkung", "Kangkung segar, cocok untuk tumis.", "ikat", 4000, 35, 1, 1},
		{"sayur", "Tomat Merah", "tomat-merah", "Tomat merah matang, segar untuk masakan.", "kg", 14000, 18000, 250, 250},
		{"sayur", "Wortel Brastagi", "wortel-brastagi", "Wortel manis dari dataran tinggi Brastagi.", "kg", 16000, 20000, 250, 250},
		{"sayur", "Kentang Dieng", "kentang-dieng", "Kentang Dieng empuk, cocok untuk sup dan perkedel.", "kg", 15000, 22000, 250, 250},
		{"sayur", "Paket Sayur Sop", "paket-sayur-sop", "Paket sayur sop lengkap: wortel, kentang, buncis, seledri.", "paket", 15000, 25, 1, 1},
	}
}

// zones mengembalikan contoh zona ongkir (dapat diubah lewat panel admin).
func zones() []zoneSeed {
	return []zoneSeed{
		{name: "Kec. Sukamaju", areas: []string{"Sukamaju", "Sukamulya"}, fee: 10000, freeMinSubtotal: 150000},
		{name: "Kec. Cibaduyut", areas: []string{"Cibaduyut", "Cibaduyut Kidul"}, fee: 12000, freeMinSubtotal: 150000},
		{name: "Kec. Dayeuhkolot", areas: []string{"Dayeuhkolot", "Citeureup"}, fee: 15000, freeMinSubtotal: 200000},
		{name: "Ambil di tempat", areas: []string{"Toko Saybu"}, fee: 0, freeMinSubtotal: 0},
	}
}

// seedZones meng-upsert zona ongkir berdasarkan nama dan mengembalikan jumlahnya.
func seedZones(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	rows := zones()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, z := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO zones (id, name, areas, fee, free_min_subtotal, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
			ON CONFLICT (name) DO UPDATE SET
				areas             = EXCLUDED.areas,
				fee               = EXCLUDED.fee,
				free_min_subtotal = EXCLUDED.free_min_subtotal,
				is_active         = true,
				updated_at        = now()
		`, ids.New(), z.name, z.areas, z.fee, z.freeMinSubtotal); err != nil {
			return 0, fmt.Errorf("zona %s: %w", z.name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// seedCategories meng-upsert kategori dan mengembalikan peta slug -> id.
func seedCategories(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	out := make(map[string]string, len(categories()))
	for _, c := range categories() {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO categories (id, name, slug, sort, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (slug) DO UPDATE
				SET name = EXCLUDED.name, sort = EXCLUDED.sort, is_active = true, updated_at = now()
			RETURNING id
		`, ids.New(), c.name, c.slug, c.sort).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("kategori %s: %w", c.slug, err)
		}
		out[c.slug] = id
	}
	return out, nil
}

// seedProducts meng-upsert produk dan mengembalikan jumlah produk.
func seedProducts(ctx context.Context, pool *pgxpool.Pool, cats map[string]string) (int, error) {
	rows := products()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, p := range rows {
		catID := cats[p.categorySlug]
		if catID == "" {
			return 0, fmt.Errorf("kategori %q tidak ditemukan untuk produk %q", p.categorySlug, p.slug)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, category_id, name, slug, description, unit,
				price, stock, min_qty, step_qty, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true)
			ON CONFLICT (slug) DO UPDATE SET
				category_id = EXCLUDED.category_id,
				name        = EXCLUDED.name,
				description = EXCLUDED.description,
				unit        = EXCLUDED.unit,
				price       = EXCLUDED.price,
				min_qty     = EXCLUDED.min_qty,
				step_qty    = EXCLUDED.step_qty,
				is_active   = true,
				updated_at  = now()
		`, ids.New(), catID, p.name, p.slug, p.description, p.unit,
			p.price, p.stock, p.minQty, p.stepQty); err != nil {
			return 0, fmt.Errorf("produk %s: %w", p.slug, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// seedAdmin memastikan akun admin demo ada. Password tidak ditimpa bila
// akun sudah ada (aman dijalankan ulang setelah admin ganti password).
func seedAdmin(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	hash, err := hashPassword("admin12345")
	if err != nil {
		return false, err
	}
	tag, err := pool.Exec(ctx, `
		INSERT INTO users (id, name, phone, email, password_hash, role)
		VALUES ($1, 'Admin Saybu', '081200000001', 'admin@saybu.test', $2, 'admin')
		ON CONFLICT DO NOTHING
	`, ids.New(), hash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// hashPassword menghasilkan hash argon2id dengan format PHC yang sama dengan
// auth service ($argon2id$v=19$m=...,t=...,p=...$salt$hash).
func hashPassword(password string) (string, error) {
	const (
		memory      = 64 * 1024
		iterations  = 3
		parallelism = 2
		saltLength  = 16
		keyLength   = 32
	)
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}
