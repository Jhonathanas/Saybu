package handlers

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	"saybu/pkg/httpx"
	"saybu/pkg/storage"
)

// CatalogService adalah bagian klien gRPC catalog yang dipakai gateway.
type CatalogService interface {
	ListCategories(ctx context.Context, in *catalogv1.ListCategoriesRequest, opts ...grpc.CallOption) (*catalogv1.ListCategoriesResponse, error)
	ListProducts(ctx context.Context, in *catalogv1.ListProductsRequest, opts ...grpc.CallOption) (*catalogv1.ListProductsResponse, error)
	GetProduct(ctx context.Context, in *catalogv1.GetProductRequest, opts ...grpc.CallOption) (*catalogv1.Product, error)
	CreateProduct(ctx context.Context, in *catalogv1.CreateProductRequest, opts ...grpc.CallOption) (*catalogv1.Product, error)
	UpdateProduct(ctx context.Context, in *catalogv1.UpdateProductRequest, opts ...grpc.CallOption) (*catalogv1.Product, error)
	SetStock(ctx context.Context, in *catalogv1.SetStockRequest, opts ...grpc.CallOption) (*catalogv1.Product, error)
	DeleteProduct(ctx context.Context, in *catalogv1.DeleteProductRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

// Catalog menangani endpoint katalog publik & admin.
type Catalog struct {
	client         CatalogService
	log            *slog.Logger
	uploads        storage.Storage
	maxUploadBytes int64
}

// NewCatalog membuat handler katalog.
func NewCatalog(client CatalogService, log *slog.Logger, uploads storage.Storage, maxUploadBytes int64) *Catalog {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 5 << 20
	}
	return &Catalog{client: client, log: log, uploads: uploads, maxUploadBytes: maxUploadBytes}
}

type categoryJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Sort int32  `json:"sort"`
}

// productJSON adalah bentuk publik produk (harga per unit, stok satuan terkecil).
type productJSON struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	Category    *categoryJSON `json:"category,omitempty"`
	Description string        `json:"description"`
	Unit        string        `json:"unit"`
	Price       int64         `json:"price"`
	Stock       int64         `json:"stock"`
	MinQty      int64         `json:"min_qty"`
	StepQty     int64         `json:"step_qty"`
	IsAvailable bool          `json:"is_available"`
	Images      []string      `json:"images"`
	CreatedAt   string        `json:"created_at,omitempty"`
	UpdatedAt   string        `json:"updated_at,omitempty"`
}

// adminProductJSON adalah bentuk produk untuk panel admin.
type adminProductJSON struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	Category    *categoryJSON `json:"category,omitempty"`
	CategoryID  string        `json:"category_id"`
	Description string        `json:"description"`
	Unit        string        `json:"unit"`
	Price       int64         `json:"price"`
	StockQty    int64         `json:"stock_qty"`
	MinQty      int64         `json:"min_qty"`
	StepQty     int64         `json:"step_qty"`
	IsActive    bool          `json:"is_active"`
	Images      []string      `json:"images"`
	CreatedAt   string        `json:"created_at,omitempty"`
	UpdatedAt   string        `json:"updated_at,omitempty"`
}

type pageMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type productBody struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	CategoryID  string   `json:"category_id"`
	Description string   `json:"description"`
	Unit        string   `json:"unit"`
	Price       int64    `json:"price"`
	StockQty    int64    `json:"stock_qty"`
	MinQty      int64    `json:"min_qty"`
	StepQty     int64    `json:"step_qty"`
	IsActive    bool     `json:"is_active"`
	Images      []string `json:"images"`
}

type productPatchBody struct {
	Name        *string  `json:"name"`
	Slug        *string  `json:"slug"`
	CategoryID  *string  `json:"category_id"`
	Description *string  `json:"description"`
	Unit        *string  `json:"unit"`
	Price       *int64   `json:"price"`
	MinQty      *int64   `json:"min_qty"`
	StepQty     *int64   `json:"step_qty"`
	IsActive    *bool    `json:"is_active"`
	Images      []string `json:"images"`
}

type stockBody struct {
	Mode string `json:"mode"`
	Qty  int64  `json:"qty"`
}

// ListCategories menangani GET /api/v1/categories.
func (h *Catalog) ListCategories(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.ListCategories(outgoingContext(r), &catalogv1.ListCategoriesRequest{})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	out := make([]categoryJSON, 0, len(res.GetCategories()))
	for _, c := range res.GetCategories() {
		out = append(out, *toCategoryJSON(c))
	}
	httpx.Data(w, http.StatusOK, out)
}

// ListProducts menangani GET /api/v1/products (hanya produk aktif).
func (h *Catalog) ListProducts(w http.ResponseWriter, r *http.Request) {
	h.listProducts(w, r, false)
}

// AdminListProducts menangani GET /api/v1/admin/products (termasuk nonaktif).
func (h *Catalog) AdminListProducts(w http.ResponseWriter, r *http.Request) {
	h.listProducts(w, r, true)
}

func (h *Catalog) listProducts(w http.ResponseWriter, r *http.Request, admin bool) {
	q := r.URL.Query()
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)

	res, err := h.client.ListProducts(outgoingContext(r), &catalogv1.ListProductsRequest{
		CategorySlug:    q.Get("category"),
		Query:           q.Get("q"),
		Sort:            q.Get("sort"),
		Page:            int32(page),
		Limit:           int32(limit),
		IncludeInactive: admin,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}

	total := res.GetTotal()
	limit = int(res.GetLimit())
	if limit < 1 {
		limit = 20
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}
	meta := pageMeta{Page: int(res.GetPage()), Limit: limit, Total: total, TotalPages: totalPages}

	if admin {
		out := make([]adminProductJSON, 0, len(res.GetProducts()))
		for _, p := range res.GetProducts() {
			out = append(out, *toAdminProductJSON(p))
		}
		httpx.DataMeta(w, http.StatusOK, out, meta)
		return
	}

	out := make([]productJSON, 0, len(res.GetProducts()))
	for _, p := range res.GetProducts() {
		out = append(out, *toProductJSON(p, true))
	}
	httpx.DataMeta(w, http.StatusOK, out, meta)
}

// GetProduct menangani GET /api/v1/products/{slug}.
func (h *Catalog) GetProduct(w http.ResponseWriter, r *http.Request) {
	slug := chiURLParam(r, "slug")
	res, err := h.client.GetProduct(outgoingContext(r), &catalogv1.GetProductRequest{Slug: slug})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toProductJSON(res, true))
}

// CreateProduct menangani POST /api/v1/admin/products.
func (h *Catalog) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var body productBody
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.client.CreateProduct(outgoingContext(r), &catalogv1.CreateProductRequest{
		Name:        body.Name,
		Slug:        body.Slug,
		CategoryId:  body.CategoryID,
		Description: body.Description,
		Unit:        body.Unit,
		Price:       body.Price,
		Stock:       body.StockQty,
		MinQty:      body.MinQty,
		StepQty:     body.StepQty,
		IsActive:    body.IsActive,
		Images:      body.Images,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusCreated, toAdminProductJSON(res))
}

// UpdateProduct menangani PATCH /api/v1/admin/products/{id}.
func (h *Catalog) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var body productPatchBody
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.client.UpdateProduct(outgoingContext(r), &catalogv1.UpdateProductRequest{
		Id:            id,
		Name:          body.Name,
		Slug:          body.Slug,
		CategoryId:    body.CategoryID,
		Description:   body.Description,
		Unit:          body.Unit,
		Price:         body.Price,
		MinQty:        body.MinQty,
		StepQty:       body.StepQty,
		IsActive:      body.IsActive,
		Images:        body.Images,
		ReplaceImages: body.Images != nil,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toAdminProductJSON(res))
}

// SetStock menangani PATCH /api/v1/admin/products/{id}/stock.
func (h *Catalog) SetStock(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var body stockBody
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.client.SetStock(outgoingContext(r), &catalogv1.SetStockRequest{
		ProductId: id,
		Mode:      body.Mode,
		Qty:       body.Qty,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toAdminProductJSON(res))
}

// DeleteProduct menangani DELETE /api/v1/admin/products/{id}.
func (h *Catalog) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	if _, err := h.client.DeleteProduct(outgoingContext(r), &catalogv1.DeleteProductRequest{Id: id}); err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Upload menangani POST /api/v1/admin/uploads (multipart/form-data, field "file").
func (h *Catalog) Upload(w http.ResponseWriter, r *http.Request) {
	if h.uploads == nil {
		httpx.WriteError(w, r, httpx.New(http.StatusServiceUnavailable, "UNAVAILABLE", "Unggah berkas tidak dikonfigurasi."))
		return
	}

	// Batasi ukuran body; beri ruang untuk overhead multipart.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(h.maxUploadBytes); err != nil {
		httpx.WriteError(w, r, httpx.BadRequest("Berkas terlalu besar atau form tidak valid."))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, httpx.BadRequest("Field 'file' wajib diisi."))
		return
	}
	defer file.Close()

	if header.Size > h.maxUploadBytes {
		httpx.WriteError(w, r, httpx.BadRequest("Ukuran berkas melebihi batas."))
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !storage.AllowedExt[ext] {
		httpx.WriteError(w, r, httpx.BadRequest("Format berkas harus JPG, PNG, atau WEBP."))
		return
	}

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	if !strings.HasPrefix(http.DetectContentType(head[:n]), "image/") {
		httpx.WriteError(w, r, httpx.BadRequest("Berkas bukan gambar yang valid."))
		return
	}
	reader := io.MultiReader(bytes.NewReader(head[:n]), file)

	url, err := h.uploads.Save(r.Context(), ext, reader)
	if err != nil {
		h.log.Error("simpan unggahan gagal", "error", err)
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}
	httpx.Data(w, http.StatusCreated, map[string]string{"url": url})
}

func toCategoryJSON(c *catalogv1.Category) *categoryJSON {
	if c == nil {
		return nil
	}
	return &categoryJSON{ID: c.GetId(), Name: c.GetName(), Slug: c.GetSlug(), Sort: c.GetSort()}
}

func toProductJSON(p *catalogv1.Product, withTimestamps bool) *productJSON {
	out := &productJSON{
		ID:          p.GetId(),
		Slug:        p.GetSlug(),
		Name:        p.GetName(),
		Category:    toCategoryJSON(p.GetCategory()),
		Description: p.GetDescription(),
		Unit:        p.GetUnit(),
		Price:       p.GetPrice(),
		Stock:       p.GetStock(),
		MinQty:      p.GetMinQty(),
		StepQty:     p.GetStepQty(),
		IsAvailable: p.GetIsActive() && p.GetStock() > 0,
		Images:      p.GetImages(),
	}
	if out.Images == nil {
		out.Images = []string{}
	}
	if withTimestamps {
		out.CreatedAt = p.GetCreatedAt()
		out.UpdatedAt = p.GetUpdatedAt()
	}
	return out
}

func toAdminProductJSON(p *catalogv1.Product) *adminProductJSON {
	out := &adminProductJSON{
		ID:          p.GetId(),
		Slug:        p.GetSlug(),
		Name:        p.GetName(),
		Category:    toCategoryJSON(p.GetCategory()),
		Description: p.GetDescription(),
		Unit:        p.GetUnit(),
		Price:       p.GetPrice(),
		StockQty:    p.GetStock(),
		MinQty:      p.GetMinQty(),
		StepQty:     p.GetStepQty(),
		IsActive:    p.GetIsActive(),
		Images:      p.GetImages(),
		CreatedAt:   p.GetCreatedAt(),
		UpdatedAt:   p.GetUpdatedAt(),
	}
	if c := p.GetCategory(); c != nil {
		out.CategoryID = c.GetId()
	}
	if out.Images == nil {
		out.Images = []string{}
	}
	return out
}

// chiURLParam mengambil parameter path dari chi.
func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}
