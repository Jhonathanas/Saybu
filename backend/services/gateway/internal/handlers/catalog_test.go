package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	"saybu/pkg/storage"
)

type fakeCatalog struct {
	listCategories func(context.Context, *catalogv1.ListCategoriesRequest) (*catalogv1.ListCategoriesResponse, error)
	listProducts   func(context.Context, *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error)
	getProduct     func(context.Context, *catalogv1.GetProductRequest) (*catalogv1.Product, error)
	createProduct  func(context.Context, *catalogv1.CreateProductRequest) (*catalogv1.Product, error)
	updateProduct  func(context.Context, *catalogv1.UpdateProductRequest) (*catalogv1.Product, error)
	setStock       func(context.Context, *catalogv1.SetStockRequest) (*catalogv1.Product, error)
	deleteProduct  func(context.Context, *catalogv1.DeleteProductRequest) (*emptypb.Empty, error)
}

func (f *fakeCatalog) ListCategories(ctx context.Context, in *catalogv1.ListCategoriesRequest, _ ...grpc.CallOption) (*catalogv1.ListCategoriesResponse, error) {
	return f.listCategories(ctx, in)
}

func (f *fakeCatalog) ListProducts(ctx context.Context, in *catalogv1.ListProductsRequest, _ ...grpc.CallOption) (*catalogv1.ListProductsResponse, error) {
	return f.listProducts(ctx, in)
}

func (f *fakeCatalog) GetProduct(ctx context.Context, in *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.Product, error) {
	return f.getProduct(ctx, in)
}

func (f *fakeCatalog) CreateProduct(ctx context.Context, in *catalogv1.CreateProductRequest, _ ...grpc.CallOption) (*catalogv1.Product, error) {
	return f.createProduct(ctx, in)
}

func (f *fakeCatalog) UpdateProduct(ctx context.Context, in *catalogv1.UpdateProductRequest, _ ...grpc.CallOption) (*catalogv1.Product, error) {
	return f.updateProduct(ctx, in)
}

func (f *fakeCatalog) SetStock(ctx context.Context, in *catalogv1.SetStockRequest, _ ...grpc.CallOption) (*catalogv1.Product, error) {
	return f.setStock(ctx, in)
}

func (f *fakeCatalog) DeleteProduct(ctx context.Context, in *catalogv1.DeleteProductRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.deleteProduct(ctx, in)
}

func newCatalogHandler(t *testing.T, client CatalogService) *Catalog {
	t.Helper()
	disk, err := storage.NewDisk(t.TempDir(), "/uploads")
	if err != nil {
		t.Fatal(err)
	}
	return NewCatalog(client, slog.New(slog.NewTextHandler(io.Discard, nil)), disk, 1<<20)
}

// withParam menaruh parameter path chi ke context request.
func withParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func sampleProduct() *catalogv1.Product {
	return &catalogv1.Product{
		Id: "p-1", Slug: "bayam-segar", Name: "Bayam Segar",
		Category:    &catalogv1.Category{Id: "c-1", Name: "Sayur", Slug: "sayur"},
		Description: "Bayam hijau segar.", Unit: "ikat", Price: 5000, Stock: 40,
		MinQty: 1, StepQty: 1, IsActive: true, Images: []string{"/uploads/a.jpg"},
		CreatedAt: "2026-11-13T10:00:00Z", UpdatedAt: "2026-11-13T10:00:00Z",
	}
}

func TestListCategories(t *testing.T) {
	h := newCatalogHandler(t, &fakeCatalog{
		listCategories: func(context.Context, *catalogv1.ListCategoriesRequest) (*catalogv1.ListCategoriesResponse, error) {
			return &catalogv1.ListCategoriesResponse{Categories: []*catalogv1.Category{
				{Id: "c-1", Name: "Buah", Slug: "buah", Sort: 1},
			}}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.ListCategories(rec, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	data, _ := decodeBody(t, rec)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data = %+v", data)
	}
	cat, _ := data[0].(map[string]any)
	if cat["slug"] != "buah" {
		t.Fatalf("kategori = %+v", cat)
	}
}

func TestListProductsPublicShapeAndMeta(t *testing.T) {
	var got *catalogv1.ListProductsRequest
	h := newCatalogHandler(t, &fakeCatalog{
		listProducts: func(_ context.Context, in *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
			got = in
			return &catalogv1.ListProductsResponse{
				Products: []*catalogv1.Product{sampleProduct()},
				Page:     1, Limit: 20, Total: 41,
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.ListProducts(rec, httptest.NewRequest(http.MethodGet, "/api/v1/products?category=sayur&q=bayam&sort=price_asc&page=1&limit=20", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetIncludeInactive() {
		t.Fatal("endpoint publik tidak boleh menyertakan produk nonaktif")
	}
	if got.GetCategorySlug() != "sayur" || got.GetQuery() != "bayam" || got.GetSort() != "price_asc" {
		t.Fatalf("request = %+v", got)
	}

	body := decodeBody(t, rec)
	meta, _ := body["meta"].(map[string]any)
	if meta["total_pages"] != float64(3) || meta["total"] != float64(41) {
		t.Fatalf("meta = %+v", meta)
	}
	item, _ := body["data"].([]any)[0].(map[string]any)
	if item["is_available"] != true {
		t.Fatalf("is_available = %v", item["is_available"])
	}
	if _, ok := item["is_active"]; ok {
		t.Fatal("respons publik tidak boleh memuat is_active")
	}
	if item["stock"] != float64(40) {
		t.Fatalf("stock = %v", item["stock"])
	}
}

func TestListProductsUnavailableWhenStockZero(t *testing.T) {
	p := sampleProduct()
	p.Stock = 0
	h := newCatalogHandler(t, &fakeCatalog{
		listProducts: func(context.Context, *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
			return &catalogv1.ListProductsResponse{Products: []*catalogv1.Product{p}, Page: 1, Limit: 20, Total: 1}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.ListProducts(rec, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	item, _ := decodeBody(t, rec)["data"].([]any)[0].(map[string]any)
	if item["is_available"] != false {
		t.Fatalf("is_available = %v", item["is_available"])
	}
}

func TestAdminListProductsIncludesInactive(t *testing.T) {
	var got *catalogv1.ListProductsRequest
	h := newCatalogHandler(t, &fakeCatalog{
		listProducts: func(_ context.Context, in *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
			got = in
			return &catalogv1.ListProductsResponse{Products: []*catalogv1.Product{sampleProduct()}, Page: 1, Limit: 20, Total: 1}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.AdminListProducts(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/products", nil))

	if !got.GetIncludeInactive() {
		t.Fatal("endpoint admin harus menyertakan produk nonaktif")
	}
	item, _ := decodeBody(t, rec)["data"].([]any)[0].(map[string]any)
	if item["stock_qty"] != float64(40) || item["is_active"] != true {
		t.Fatalf("item = %+v", item)
	}
	if item["category_id"] != "c-1" {
		t.Fatalf("category_id = %v", item["category_id"])
	}
}

func TestGetProductBySlug(t *testing.T) {
	var gotSlug string
	h := newCatalogHandler(t, &fakeCatalog{
		getProduct: func(_ context.Context, in *catalogv1.GetProductRequest) (*catalogv1.Product, error) {
			gotSlug = in.GetSlug()
			return sampleProduct(), nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/v1/products/bayam-segar", nil), "slug", "bayam-segar")
	h.GetProduct(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if gotSlug != "bayam-segar" {
		t.Fatalf("slug = %q", gotSlug)
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	if data["created_at"] == "" {
		t.Fatal("detail harus memuat created_at")
	}
}

func TestGetProductNotFound(t *testing.T) {
	h := newCatalogHandler(t, &fakeCatalog{
		getProduct: func(context.Context, *catalogv1.GetProductRequest) (*catalogv1.Product, error) {
			return nil, status.Error(codes.NotFound, "Data tidak ditemukan.")
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/v1/products/x", nil), "slug", "x")
	h.GetProduct(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestCreateProduct(t *testing.T) {
	var got *catalogv1.CreateProductRequest
	h := newCatalogHandler(t, &fakeCatalog{
		createProduct: func(_ context.Context, in *catalogv1.CreateProductRequest) (*catalogv1.Product, error) {
			got = in
			return sampleProduct(), nil
		},
	})

	payload := `{"name":"Bayam Segar","slug":"bayam-segar","category_id":"c-1","unit":"ikat",` +
		`"price":5000,"stock_qty":40,"min_qty":1,"step_qty":1,"is_active":true,"images":["/uploads/a.jpg"]}`
	rec := httptest.NewRecorder()
	h.CreateProduct(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(payload)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if got.GetStock() != 40 || got.GetCategoryId() != "c-1" || got.GetPrice() != 5000 {
		t.Fatalf("request = %+v", got)
	}
}

func TestUpdateProductPartialKeepsImages(t *testing.T) {
	var got *catalogv1.UpdateProductRequest
	h := newCatalogHandler(t, &fakeCatalog{
		updateProduct: func(_ context.Context, in *catalogv1.UpdateProductRequest) (*catalogv1.Product, error) {
			got = in
			return sampleProduct(), nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/p-1",
		strings.NewReader(`{"price":6000}`)), "id", "p-1")
	h.UpdateProduct(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetPrice() != 6000 || got.Name != nil || got.IsActive != nil {
		t.Fatalf("request = %+v", got)
	}
	if got.GetReplaceImages() {
		t.Fatal("images tidak dikirim: replace_images harus false")
	}
}

func TestUpdateProductReplaceImages(t *testing.T) {
	var got *catalogv1.UpdateProductRequest
	h := newCatalogHandler(t, &fakeCatalog{
		updateProduct: func(_ context.Context, in *catalogv1.UpdateProductRequest) (*catalogv1.Product, error) {
			got = in
			return sampleProduct(), nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/p-1",
		strings.NewReader(`{"images":["/uploads/b.jpg"]}`)), "id", "p-1")
	h.UpdateProduct(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !got.GetReplaceImages() || len(got.GetImages()) != 1 {
		t.Fatalf("request = %+v", got)
	}
}

func TestSetStock(t *testing.T) {
	var got *catalogv1.SetStockRequest
	h := newCatalogHandler(t, &fakeCatalog{
		setStock: func(_ context.Context, in *catalogv1.SetStockRequest) (*catalogv1.Product, error) {
			got = in
			return sampleProduct(), nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/p-1/stock",
		strings.NewReader(`{"mode":"adjust","qty":-5}`)), "id", "p-1")
	h.SetStock(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetMode() != "adjust" || got.GetQty() != -5 || got.GetProductId() != "p-1" {
		t.Fatalf("request = %+v", got)
	}
}

func TestSetStockNegativeMaps422(t *testing.T) {
	h := newCatalogHandler(t, &fakeCatalog{
		setStock: func(context.Context, *catalogv1.SetStockRequest) (*catalogv1.Product, error) {
			return nil, status.Error(codes.FailedPrecondition, "Stok tidak boleh negatif.")
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/p-1/stock",
		strings.NewReader(`{"mode":"set","qty":-1}`)), "id", "p-1")
	h.SetStock(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestDeleteProduct(t *testing.T) {
	var gotID string
	h := newCatalogHandler(t, &fakeCatalog{
		deleteProduct: func(_ context.Context, in *catalogv1.DeleteProductRequest) (*emptypb.Empty, error) {
			gotID = in.GetId()
			return &emptypb.Empty{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/p-1", nil), "id", "p-1")
	h.DeleteProduct(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if gotID != "p-1" {
		t.Fatalf("id = %q", gotID)
	}
}

func TestUploadStoresImage(t *testing.T) {
	h := newCatalogHandler(t, &fakeCatalog{})

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "foto.png")
	if err != nil {
		t.Fatal(err)
	}
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0}, 64)...)
	if _, err := fw.Write(png); err != nil {
		t.Fatal(err)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/uploads", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Upload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	url, _ := data["url"].(string)
	if !strings.HasPrefix(url, "/uploads/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("url = %q", url)
	}
}

func TestUploadRejectsNonImage(t *testing.T) {
	h := newCatalogHandler(t, &fakeCatalog{})

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, _ := mw.CreateFormFile("file", "dokumen.txt")
	_, _ = fw.Write([]byte("halo, ini teks biasa"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/uploads", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadRejectsMissingField(t *testing.T) {
	h := newCatalogHandler(t, &fakeCatalog{})

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/uploads", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCatalogRejectsUnknownFields(t *testing.T) {
	h := newCatalogHandler(t, &fakeCatalog{})

	rec := httptest.NewRecorder()
	h.CreateProduct(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/products",
		strings.NewReader(`{"name":"X","tidak_dikenal":1}`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
}
