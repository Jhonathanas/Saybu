// Package service berisi usecase katalog: produk, kategori, dan stok.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"saybu/pkg/ids"
	"saybu/pkg/kafkax"
	"saybu/pkg/outbox"
	"saybu/services/catalog/internal/domain"
)

// Nama topik dan producer untuk event katalog.
const (
	TopicCatalogEvents = "catalog.events.v1"
	producerName       = "catalog-svc"
)

// Repo adalah kebutuhan service terhadap database.
type Repo interface {
	ListCategories(ctx context.Context, includeInactive bool) ([]domain.Category, error)
	GetCategory(ctx context.Context, id string) (domain.Category, error)
	ListProducts(ctx context.Context, f domain.ProductFilter) ([]domain.Product, int64, error)
	GetProductByID(ctx context.Context, id string, includeInactive bool) (domain.Product, error)
	GetProductBySlug(ctx context.Context, slug string, includeInactive bool) (domain.Product, error)
	CreateProduct(ctx context.Context, p domain.Product, events []outbox.Record) (domain.Product, error)
	UpdateProduct(ctx context.Context, p domain.Product, events []outbox.Record) (domain.Product, error)
	SetStock(ctx context.Context, id string, newStock int64, events []outbox.Record) (domain.Product, error)
	SetActive(ctx context.Context, id string, active bool, events []outbox.Record) (domain.Product, error)
	CheckStock(ctx context.Context, items []domain.StockItem) ([]domain.StockStatus, error)
	ReserveStock(ctx context.Context, orderID string, items []domain.StockItem) ([]domain.StockStatus, error)
	ReleaseStock(ctx context.Context, orderID string) error
	ReleaseStockOnce(ctx context.Context, eventID, orderID string) (bool, error)
	ConfirmStockOnce(ctx context.Context, eventID, orderID string) (bool, error)
}

// Service adalah usecase katalog.
type Service struct {
	repo Repo
}

// New membuat Service.
func New(repo Repo) (*Service, error) {
	if repo == nil {
		return nil, errors.New("service: repo wajib")
	}
	return &Service{repo: repo}, nil
}

// ListProductsInput adalah parameter daftar produk.
type ListProductsInput struct {
	CategorySlug    string
	Query           string
	Sort            string
	Page            int
	Limit           int
	IncludeInactive bool
}

// CreateProductInput adalah input pembuatan produk.
type CreateProductInput struct {
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

// UpdateProductInput adalah patch produk; hanya field terisi yang diubah.
type UpdateProductInput struct {
	ID            string
	Name          *string
	Slug          *string
	CategoryID    *string
	Description   *string
	Unit          *string
	Price         *int64
	MinQty        *int64
	StepQty       *int64
	IsActive      *bool
	Images        []string
	ReplaceImages bool
}

// ListCategories mengambil daftar kategori.
func (s *Service) ListCategories(ctx context.Context, includeInactive bool) ([]domain.Category, error) {
	return s.repo.ListCategories(ctx, includeInactive)
}

// ListProducts mengambil daftar produk dengan filter & paginasi.
func (s *Service) ListProducts(ctx context.Context, in ListProductsInput) ([]domain.Product, int64, int, int, error) {
	page, limit := normalizePage(in.Page, in.Limit)
	products, total, err := s.repo.ListProducts(ctx, domain.ProductFilter{
		CategorySlug:    strings.TrimSpace(in.CategorySlug),
		Query:           strings.TrimSpace(in.Query),
		Sort:            in.Sort,
		Page:            page,
		Limit:           limit,
		IncludeInactive: in.IncludeInactive,
	})
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return products, total, page, limit, nil
}

// GetProduct mengambil produk berdasarkan id atau slug.
func (s *Service) GetProduct(ctx context.Context, id, slug string, includeInactive bool) (domain.Product, error) {
	switch {
	case strings.TrimSpace(id) != "":
		return s.repo.GetProductByID(ctx, strings.TrimSpace(id), includeInactive)
	case strings.TrimSpace(slug) != "":
		return s.repo.GetProductBySlug(ctx, strings.TrimSpace(slug), includeInactive)
	default:
		return domain.Product{}, (&domain.ValidationError{}).
			Add("id", "ID atau slug produk wajib diisi.")
	}
}

// CreateProduct membuat produk baru.
func (s *Service) CreateProduct(ctx context.Context, in CreateProductInput) (domain.Product, error) {
	clean, err := domain.ValidateProduct(domain.ProductInput{
		Name:        in.Name,
		Slug:        in.Slug,
		CategoryID:  in.CategoryID,
		Description: in.Description,
		Unit:        in.Unit,
		Price:       in.Price,
		Stock:       in.Stock,
		MinQty:      in.MinQty,
		StepQty:     in.StepQty,
		IsActive:    in.IsActive,
		Images:      in.Images,
	})
	if err != nil {
		return domain.Product{}, err
	}
	if err := s.ensureCategory(ctx, clean.CategoryID); err != nil {
		return domain.Product{}, err
	}

	p := domain.Product{
		ID:          ids.New(),
		Name:        clean.Name,
		Slug:        clean.Slug,
		Description: clean.Description,
		Unit:        clean.Unit,
		Price:       clean.Price,
		Stock:       clean.Stock,
		MinQty:      clean.MinQty,
		StepQty:     clean.StepQty,
		IsActive:    clean.IsActive,
		Images:      clean.Images,
	}
	if clean.CategoryID != "" {
		p.Category = &domain.Category{ID: clean.CategoryID}
	}
	domain.ApplyDefaults(&p)

	events, err := s.productUpdatedEvents(p)
	if err != nil {
		return domain.Product{}, err
	}
	return s.repo.CreateProduct(ctx, p, events)
}

// UpdateProduct menerapkan patch pada produk yang ada.
func (s *Service) UpdateProduct(ctx context.Context, in UpdateProductInput) (domain.Product, error) {
	if strings.TrimSpace(in.ID) == "" {
		return domain.Product{}, (&domain.ValidationError{}).Add("id", "ID produk wajib diisi.")
	}
	existing, err := s.repo.GetProductByID(ctx, in.ID, true)
	if err != nil {
		return domain.Product{}, err
	}

	if in.Name != nil {
		existing.Name = *in.Name
	}
	if in.Slug != nil {
		existing.Slug = *in.Slug
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if in.Unit != nil {
		existing.Unit = *in.Unit
	}
	if in.Price != nil {
		existing.Price = *in.Price
	}
	if in.MinQty != nil {
		existing.MinQty = *in.MinQty
	}
	if in.StepQty != nil {
		existing.StepQty = *in.StepQty
	}
	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}
	if in.ReplaceImages {
		existing.Images = in.Images
	}

	clean, err := domain.ValidateProduct(domain.ProductInput{
		Name:        existing.Name,
		Slug:        existing.Slug,
		Description: existing.Description,
		Unit:        existing.Unit,
		Price:       existing.Price,
		Stock:       existing.Stock,
		MinQty:      existing.MinQty,
		StepQty:     existing.StepQty,
		IsActive:    existing.IsActive,
		Images:      existing.Images,
	})
	if err != nil {
		return domain.Product{}, err
	}
	existing.Name = clean.Name
	existing.Slug = clean.Slug
	existing.Description = clean.Description
	existing.Unit = clean.Unit
	existing.Price = clean.Price
	existing.MinQty = clean.MinQty
	existing.StepQty = clean.StepQty
	domain.ApplyDefaults(&existing)

	if in.CategoryID != nil {
		catID := strings.TrimSpace(*in.CategoryID)
		if err := s.ensureCategory(ctx, catID); err != nil {
			return domain.Product{}, err
		}
		if catID == "" {
			existing.Category = nil
		} else {
			existing.Category = &domain.Category{ID: catID}
		}
	}

	events, err := s.productUpdatedEvents(existing)
	if err != nil {
		return domain.Product{}, err
	}
	return s.repo.UpdateProduct(ctx, existing, events)
}

// SetStock mengubah stok dengan mode "set" atau "adjust".
func (s *Service) SetStock(ctx context.Context, id, mode string, qty int64) (domain.Product, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Product{}, (&domain.ValidationError{}).Add("product_id", "ID produk wajib diisi.")
	}
	existing, err := s.repo.GetProductByID(ctx, id, true)
	if err != nil {
		return domain.Product{}, err
	}

	var newStock int64
	switch mode {
	case "", "set":
		newStock = qty
	case "adjust":
		newStock = existing.Stock + qty
	default:
		return domain.Product{}, (&domain.ValidationError{}).Add("mode", "Mode harus 'set' atau 'adjust'.")
	}
	if newStock < 0 {
		return domain.Product{}, domain.ErrStockNegative
	}

	var events []outbox.Record
	if existing.Stock > existing.LowStockThreshold && newStock <= existing.LowStockThreshold {
		ev, err := s.stockLowEvent(existing, newStock)
		if err != nil {
			return domain.Product{}, err
		}
		events = append(events, ev)
	}
	return s.repo.SetStock(ctx, id, newStock, events)
}

// DeleteProduct melakukan soft delete (is_active=false).
func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return (&domain.ValidationError{}).Add("id", "ID produk wajib diisi.")
	}
	existing, err := s.repo.GetProductByID(ctx, id, true)
	if err != nil {
		return err
	}
	existing.IsActive = false
	events, err := s.productUpdatedEvents(existing)
	if err != nil {
		return err
	}
	_, err = s.repo.SetActive(ctx, id, false, events)
	return err
}

// CheckStock melaporkan ketersediaan stok.
func (s *Service) CheckStock(ctx context.Context, items []domain.StockItem) ([]domain.StockStatus, error) {
	if len(items) == 0 {
		return nil, (&domain.ValidationError{}).Add("items", "Daftar item tidak boleh kosong.")
	}
	return s.repo.CheckStock(ctx, items)
}

// ReserveStock mengurangi stok untuk sebuah order.
func (s *Service) ReserveStock(ctx context.Context, orderID string, items []domain.StockItem) ([]domain.StockStatus, error) {
	if strings.TrimSpace(orderID) == "" {
		return nil, (&domain.ValidationError{}).Add("order_id", "ID order wajib diisi.")
	}
	if len(items) == 0 {
		return nil, (&domain.ValidationError{}).Add("items", "Daftar item tidak boleh kosong.")
	}
	return s.repo.ReserveStock(ctx, orderID, items)
}

// ReleaseStock mengembalikan stok reservasi order.
func (s *Service) ReleaseStock(ctx context.Context, orderID string) error {
	if strings.TrimSpace(orderID) == "" {
		return (&domain.ValidationError{}).Add("order_id", "ID order wajib diisi.")
	}
	return s.repo.ReleaseStock(ctx, orderID)
}

// ReleaseStockForOrder melepas stok untuk event pesanan secara idempotent
// (event_id dicatat di processed_events dalam transaksi yang sama).
func (s *Service) ReleaseStockForOrder(ctx context.Context, eventID, orderID string) error {
	if strings.TrimSpace(eventID) == "" {
		return (&domain.ValidationError{}).Add("event_id", "ID event wajib diisi.")
	}
	if strings.TrimSpace(orderID) == "" {
		return (&domain.ValidationError{}).Add("order_id", "ID order wajib diisi.")
	}
	_, err := s.repo.ReleaseStockOnce(ctx, eventID, orderID)
	return err
}

// HandleOrderEvent memproses event dari order.events.v1.
//
// order.cancelled dan order.expired melepas reservasi stok pesanan. Tipe lain
// diabaikan. Handler idempotent lewat processed_events.
func (s *Service) HandleOrderEvent(ctx context.Context, ev kafkax.Event) error {
	switch ev.Type {
	case "order.cancelled", "order.expired":
		var p struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("service: payload %s tidak valid: %w", ev.Type, err)
		}
		if strings.TrimSpace(p.OrderID) == "" {
			return fmt.Errorf("service: event %s tanpa order_id", ev.Type)
		}
		return s.ReleaseStockForOrder(ctx, ev.EventID, p.OrderID)
	default:
		return nil
	}
}

// ConfirmStockForOrder menandai reservasi stok CONFIRMED setelah pembayaran
// terverifikasi, idempotent lewat processed_events.
func (s *Service) ConfirmStockForOrder(ctx context.Context, eventID, orderID string) error {
	if strings.TrimSpace(eventID) == "" {
		return (&domain.ValidationError{}).Add("event_id", "ID event wajib diisi.")
	}
	if strings.TrimSpace(orderID) == "" {
		return (&domain.ValidationError{}).Add("order_id", "ID order wajib diisi.")
	}
	_, err := s.repo.ConfirmStockOnce(ctx, eventID, orderID)
	return err
}

// HandlePaymentEvent memproses event dari payment.events.v1.
//
// payment.confirmed menandai reservasi stok pesanan sebagai CONFIRMED
// (pemakaian final). Tipe lain diabaikan.
func (s *Service) HandlePaymentEvent(ctx context.Context, ev kafkax.Event) error {
	switch ev.Type {
	case "payment.confirmed":
		var p struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("service: payload %s tidak valid: %w", ev.Type, err)
		}
		if strings.TrimSpace(p.OrderID) == "" {
			return fmt.Errorf("service: event %s tanpa order_id", ev.Type)
		}
		return s.ConfirmStockForOrder(ctx, ev.EventID, p.OrderID)
	default:
		return nil
	}
}

func (s *Service) ensureCategory(ctx context.Context, categoryID string) error {
	if strings.TrimSpace(categoryID) == "" {
		return nil
	}
	_, err := s.repo.GetCategory(ctx, categoryID)
	return err
}

func (s *Service) productUpdatedEvents(p domain.Product) ([]outbox.Record, error) {
	payload := map[string]any{
		"product_id": p.ID,
		"name":       p.Name,
		"slug":       p.Slug,
		"price":      p.Price,
		"unit":       p.Unit,
		"is_active":  p.IsActive,
	}
	rec, err := newRecord("product.updated", p.ID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func (s *Service) stockLowEvent(p domain.Product, remaining int64) (outbox.Record, error) {
	payload := map[string]any{
		"product_id": p.ID,
		"name":       p.Name,
		"remaining":  remaining,
		"unit":       p.Unit,
		"threshold":  p.LowStockThreshold,
	}
	return newRecord("stock.low", p.ID, payload)
}

func newRecord(eventType, key string, payload any) (outbox.Record, error) {
	ev, err := kafkax.NewEvent(eventType, key, payload, producerName)
	if err != nil {
		return outbox.Record{}, fmt.Errorf("service: bangun event %s: %w", eventType, err)
	}
	raw, err := ev.Marshal()
	if err != nil {
		return outbox.Record{}, fmt.Errorf("service: marshal event %s: %w", eventType, err)
	}
	return outbox.Record{
		EventID: ev.EventID,
		Topic:   TopicCatalogEvents,
		Key:     key,
		Payload: raw,
		Headers: map[string]string{"event-type": eventType},
	}, nil
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
