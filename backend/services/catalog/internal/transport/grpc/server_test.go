package grpc

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	"saybu/services/catalog/internal/domain"
	"saybu/services/catalog/internal/service"
)

type stubService struct {
	cats      []domain.Category
	catsErr   error
	products  []domain.Product
	total     int64
	listErr   error
	product   domain.Product
	prodErr   error
	stock     []domain.StockStatus
	stockErr  error
	created   domain.Product
	createErr error
}

func (s *stubService) ListCategories(context.Context, bool) ([]domain.Category, error) {
	return s.cats, s.catsErr
}
func (s *stubService) ListProducts(context.Context, service.ListProductsInput) ([]domain.Product, int64, int, int, error) {
	return s.products, s.total, 1, 20, s.listErr
}
func (s *stubService) GetProduct(context.Context, string, string, bool) (domain.Product, error) {
	return s.product, s.prodErr
}
func (s *stubService) CreateProduct(context.Context, service.CreateProductInput) (domain.Product, error) {
	return s.created, s.createErr
}
func (s *stubService) UpdateProduct(context.Context, service.UpdateProductInput) (domain.Product, error) {
	return s.product, s.prodErr
}
func (s *stubService) SetStock(context.Context, string, string, int64) (domain.Product, error) {
	return s.product, s.prodErr
}
func (s *stubService) DeleteProduct(context.Context, string) error { return s.prodErr }
func (s *stubService) CheckStock(context.Context, []domain.StockItem) ([]domain.StockStatus, error) {
	return s.stock, s.stockErr
}
func (s *stubService) ReserveStock(context.Context, string, []domain.StockItem) ([]domain.StockStatus, error) {
	return s.stock, s.stockErr
}
func (s *stubService) ReleaseStock(context.Context, string) error { return s.stockErr }

func startServer(t *testing.T, svc Service) catalogv1.CatalogServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	Register(srv, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return catalogv1.NewCatalogServiceClient(conn)
}

func TestListProducts(t *testing.T) {
	created := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	stub := &stubService{
		products: []domain.Product{{
			ID: "p1", Slug: "tomat", Name: "Tomat", Unit: domain.UnitKg,
			Price: 15000, Stock: 5000, MinQty: 100, StepQty: 100, IsActive: true,
			Category:  &domain.Category{ID: "c1", Name: "Sayur", Slug: "sayur"},
			Images:    []string{"/uploads/a.jpg"},
			CreatedAt: created, UpdatedAt: created,
		}},
		total: 1,
	}
	client := startServer(t, stub)

	resp, err := client.ListProducts(context.Background(), &catalogv1.ListProductsRequest{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTotal() != 1 || len(resp.GetProducts()) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	p := resp.GetProducts()[0]
	if p.GetUnit() != "kg" || p.GetPrice() != 15000 || p.GetCategory().GetSlug() != "sayur" {
		t.Fatalf("product = %+v", p)
	}
}

func TestListCategories(t *testing.T) {
	stub := &stubService{cats: []domain.Category{{ID: "c1", Name: "Buah", Slug: "buah", Sort: 1, IsActive: true}}}
	client := startServer(t, stub)

	resp, err := client.ListCategories(context.Background(), &catalogv1.ListCategoriesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetCategories()) != 1 || resp.GetCategories()[0].GetSlug() != "buah" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestGetProductNotFound(t *testing.T) {
	stub := &stubService{prodErr: domain.ErrProductNotFound}
	client := startServer(t, stub)

	_, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Slug: "x"})
	if st, _ := status.FromError(err); st.Code() != codes.NotFound {
		t.Fatalf("code = %v", st.Code())
	}
}

func TestCreateProductValidation(t *testing.T) {
	stub := &stubService{createErr: &domain.ValidationError{Errors: []domain.FieldError{
		{Field: "price", Message: "Harga tidak boleh negatif."},
	}}}
	client := startServer(t, stub)

	_, err := client.CreateProduct(context.Background(), &catalogv1.CreateProductRequest{Name: "X"})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
	}
	found := false
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				if v.GetField() == "price" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("detail price tidak ada: %+v", st.Details())
	}
}

func TestSetStockNegative(t *testing.T) {
	stub := &stubService{prodErr: domain.ErrStockNegative}
	client := startServer(t, stub)

	_, err := client.SetStock(context.Background(), &catalogv1.SetStockRequest{ProductId: "p1", Mode: "set", Qty: -1})
	if st, _ := status.FromError(err); st.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %v", st.Code())
	}
}

func TestCheckStockAllOk(t *testing.T) {
	stub := &stubService{stock: []domain.StockStatus{
		{ProductID: "p1", Name: "Tomat", Requested: 1000, Available: 5000, OK: true},
	}}
	client := startServer(t, stub)

	resp, err := client.CheckStock(context.Background(), &catalogv1.CheckStockRequest{
		Items: []*catalogv1.StockItem{{ProductId: "p1", Qty: 1000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetAllOk() || len(resp.GetItems()) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
}
