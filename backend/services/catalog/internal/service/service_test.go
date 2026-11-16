package service

import (
	"context"
	"errors"
	"testing"

	"saybu/pkg/outbox"
	"saybu/services/catalog/internal/domain"
)

type fakeRepo struct {
	products    map[string]domain.Product
	cats        map[string]domain.Category
	lastEv      []outbox.Record
	lastPage    int
	lastLim     int
	created     []domain.Product
	releaseOnce []string
	confirmOnce []string
	crossed     []domain.Product
	lastLimit   int
}

func newFake() *fakeRepo {
	return &fakeRepo{
		products: map[string]domain.Product{},
		cats:     map[string]domain.Category{"cat-1": {ID: "cat-1", Name: "Buah", Slug: "buah"}},
	}
}

func (f *fakeRepo) ListCategories(context.Context, bool) ([]domain.Category, error) {
	return []domain.Category{{ID: "cat-1", Name: "Buah", Slug: "buah"}}, nil
}
func (f *fakeRepo) GetCategory(_ context.Context, id string) (domain.Category, error) {
	c, ok := f.cats[id]
	if !ok {
		return domain.Category{}, domain.ErrCategoryNotFound
	}
	return c, nil
}
func (f *fakeRepo) ListProducts(_ context.Context, in domain.ProductFilter) ([]domain.Product, int64, error) {
	f.lastPage, f.lastLim = in.Page, in.Limit
	var out []domain.Product
	for _, p := range f.products {
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}
func (f *fakeRepo) ListLowStock(_ context.Context, limit int) ([]domain.Product, error) {
	f.lastLimit = limit
	var out []domain.Product
	for _, p := range f.products {
		if p.IsActive && p.Stock <= p.LowStockThreshold {
			out = append(out, p)
		}
	}
	return out, nil
}
func (f *fakeRepo) GetProductByID(_ context.Context, id string, _ bool) (domain.Product, error) {
	p, ok := f.products[id]
	if !ok {
		return domain.Product{}, domain.ErrProductNotFound
	}
	return p, nil
}
func (f *fakeRepo) GetProductBySlug(_ context.Context, slug string, _ bool) (domain.Product, error) {
	for _, p := range f.products {
		if p.Slug == slug {
			return p, nil
		}
	}
	return domain.Product{}, domain.ErrProductNotFound
}
func (f *fakeRepo) CreateProduct(_ context.Context, p domain.Product, ev []outbox.Record) (domain.Product, error) {
	f.products[p.ID] = p
	f.lastEv = ev
	f.created = append(f.created, p)
	return p, nil
}
func (f *fakeRepo) UpdateProduct(_ context.Context, p domain.Product, ev []outbox.Record) (domain.Product, error) {
	f.products[p.ID] = p
	f.lastEv = ev
	return p, nil
}
func (f *fakeRepo) SetStock(_ context.Context, id string, stock int64, ev []outbox.Record) (domain.Product, error) {
	p := f.products[id]
	p.Stock = stock
	f.products[id] = p
	f.lastEv = ev
	return p, nil
}
func (f *fakeRepo) SetActive(_ context.Context, id string, active bool, ev []outbox.Record) (domain.Product, error) {
	p := f.products[id]
	p.IsActive = active
	f.products[id] = p
	f.lastEv = ev
	return p, nil
}
func (f *fakeRepo) CheckStock(context.Context, []domain.StockItem) ([]domain.StockStatus, error) {
	return nil, nil
}
func (f *fakeRepo) ReserveStock(_ context.Context, _ string, _ []domain.StockItem, events func([]domain.Product) ([]outbox.Record, error)) ([]domain.StockStatus, error) {
	if events != nil && len(f.crossed) > 0 {
		recs, err := events(f.crossed)
		if err != nil {
			return nil, err
		}
		f.lastEv = recs
	}
	return nil, nil
}
func (f *fakeRepo) ReleaseStock(context.Context, string) error { return nil }
func (f *fakeRepo) ReleaseStockOnce(_ context.Context, eventID, orderID string) (bool, error) {
	f.releaseOnce = append(f.releaseOnce, eventID+":"+orderID)
	return true, nil
}
func (f *fakeRepo) ConfirmStockOnce(_ context.Context, eventID, orderID string) (bool, error) {
	f.confirmOnce = append(f.confirmOnce, eventID+":"+orderID)
	return true, nil
}

func newService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	r := newFake()
	s, err := New(r)
	if err != nil {
		t.Fatal(err)
	}
	return s, r
}

func TestCreateProduct(t *testing.T) {
	s, r := newService(t)

	p, err := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat Merah", Unit: domain.UnitKg, Price: 15000, Stock: 5000,
		CategoryID: "cat-1", IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "tomat-merah" {
		t.Fatalf("slug = %q", p.Slug)
	}
	if p.MinQty != 100 || p.StepQty != 100 {
		t.Fatalf("defaults = %d/%d", p.MinQty, p.StepQty)
	}
	if len(r.lastEv) != 1 || r.lastEv[0].Topic != TopicCatalogEvents {
		t.Fatalf("event = %+v", r.lastEv)
	}
}

func TestCreateProductUnknownCategory(t *testing.T) {
	s, _ := newService(t)
	_, err := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat", Unit: domain.UnitKg, Price: 1, CategoryID: "nope",
	})
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateProductInvalid(t *testing.T) {
	s, _ := newService(t)
	_, err := s.CreateProduct(context.Background(), CreateProductInput{Name: "A", Price: -1})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestUpdateProductPatch(t *testing.T) {
	s, r := newService(t)
	created, _ := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat", Unit: domain.UnitKg, Price: 10000, Stock: 1000, IsActive: true,
	})

	newName := "Tomat Merah"
	newPrice := int64(12000)
	updated, err := s.UpdateProduct(context.Background(), UpdateProductInput{
		ID: created.ID, Name: &newName, Price: &newPrice,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Tomat Merah" || updated.Price != 12000 {
		t.Fatalf("updated = %+v", updated)
	}
	// Slug lama tetap karena tidak diubah.
	if updated.Slug != "tomat" {
		t.Fatalf("slug = %q", updated.Slug)
	}
	if len(r.lastEv) != 1 {
		t.Fatalf("event = %+v", r.lastEv)
	}
}

func TestSetStockAdjustAndLowEvent(t *testing.T) {
	s, r := newService(t)
	created, _ := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat", Unit: domain.UnitKg, Price: 10000, Stock: 10000, IsActive: true,
	})
	r.products[created.ID] = withThreshold(r.products[created.ID], 5000)

	// 10000 -> 4000, melewati ambang 5000: event stock.low.
	p, err := s.SetStock(context.Background(), created.ID, "adjust", -6000)
	if err != nil {
		t.Fatal(err)
	}
	if p.Stock != 4000 {
		t.Fatalf("stock = %d", p.Stock)
	}
	if len(r.lastEv) != 1 || r.lastEv[0].Headers["event-type"] != "stock.low" {
		t.Fatalf("event = %+v", r.lastEv)
	}
}

func TestSetStockNegativeRejected(t *testing.T) {
	s, _ := newService(t)
	created, _ := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat", Unit: domain.UnitKg, Price: 1, Stock: 1000, IsActive: true,
	})
	_, err := s.SetStock(context.Background(), created.ID, "adjust", -5000)
	if !errors.Is(err, domain.ErrStockNegative) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteProductSoft(t *testing.T) {
	s, r := newService(t)
	created, _ := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat", Unit: domain.UnitKg, Price: 1, Stock: 1000, IsActive: true,
	})
	if err := s.DeleteProduct(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if r.products[created.ID].IsActive {
		t.Fatal("produk harus nonaktif")
	}
}

func TestGetProductRequiresIdentifier(t *testing.T) {
	s, _ := newService(t)
	_, err := s.GetProduct(context.Background(), "", "", false)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v", err)
	}
}

func TestListProductsNormalizesPaging(t *testing.T) {
	s, r := newService(t)
	_, _, page, limit, err := s.ListProducts(context.Background(), ListProductsInput{Page: 0, Limit: 999})
	if err != nil {
		t.Fatal(err)
	}
	if page != 1 || limit != 100 {
		t.Fatalf("page/limit = %d/%d", page, limit)
	}
	if r.lastPage != 1 || r.lastLim != 100 {
		t.Fatalf("repo page/limit = %d/%d", r.lastPage, r.lastLim)
	}
}

func withThreshold(p domain.Product, t int64) domain.Product {
	p.LowStockThreshold = t
	return p
}

func TestReserveStockEmitsLowEventOnCrossing(t *testing.T) {
	s, r := newService(t)
	created, _ := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat", Unit: domain.UnitKg, Price: 10000, Stock: 4000, IsActive: true,
	})
	r.products[created.ID] = withThreshold(r.products[created.ID], 5000)
	r.crossed = []domain.Product{{ID: created.ID, Name: "Tomat", Unit: domain.UnitKg, Stock: 3000, LowStockThreshold: 5000}}

	if _, err := s.ReserveStock(context.Background(), "o-1", []domain.StockItem{{ProductID: created.ID, Qty: 1000}}); err != nil {
		t.Fatal(err)
	}
	if len(r.lastEv) != 1 || r.lastEv[0].Headers["event-type"] != "stock.low" {
		t.Fatalf("event = %+v", r.lastEv)
	}
}

func TestListLowStockDelegates(t *testing.T) {
	s, r := newService(t)
	created, _ := s.CreateProduct(context.Background(), CreateProductInput{
		Name: "Tomat", Unit: domain.UnitKg, Price: 10000, Stock: 3000, IsActive: true,
	})
	r.products[created.ID] = withThreshold(r.products[created.ID], 5000)

	out, err := s.ListLowStock(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != created.ID {
		t.Fatalf("out = %+v", out)
	}
	if r.lastLimit != 5 {
		t.Fatalf("limit = %d", r.lastLimit)
	}
}
