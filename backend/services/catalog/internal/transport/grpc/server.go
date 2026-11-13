// Package grpc mengimplementasikan proto catalog.v1.CatalogService.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	"saybu/services/catalog/internal/domain"
	"saybu/services/catalog/internal/service"
)

// Service adalah usecase yang dibutuhkan transport.
type Service interface {
	ListCategories(ctx context.Context, includeInactive bool) ([]domain.Category, error)
	ListProducts(ctx context.Context, in service.ListProductsInput) ([]domain.Product, int64, int, int, error)
	GetProduct(ctx context.Context, id, slug string, includeInactive bool) (domain.Product, error)
	CreateProduct(ctx context.Context, in service.CreateProductInput) (domain.Product, error)
	UpdateProduct(ctx context.Context, in service.UpdateProductInput) (domain.Product, error)
	SetStock(ctx context.Context, id, mode string, qty int64) (domain.Product, error)
	DeleteProduct(ctx context.Context, id string) error
	CheckStock(ctx context.Context, items []domain.StockItem) ([]domain.StockStatus, error)
	ReserveStock(ctx context.Context, orderID string, items []domain.StockItem) ([]domain.StockStatus, error)
	ReleaseStock(ctx context.Context, orderID string) error
}

// Server membungkus usecase katalog sebagai gRPC.
type Server struct {
	catalogv1.UnimplementedCatalogServiceServer
	svc Service
	log *slog.Logger
}

// New membuat Server.
func New(svc Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{svc: svc, log: log}
}

// Register mendaftarkan CatalogService ke gRPC server.
func Register(srv *googlegrpc.Server, svc Service, log *slog.Logger) {
	catalogv1.RegisterCatalogServiceServer(srv, New(svc, log))
}

// ListCategories mengembalikan daftar kategori.
func (s *Server) ListCategories(ctx context.Context, req *catalogv1.ListCategoriesRequest) (*catalogv1.ListCategoriesResponse, error) {
	cats, err := s.svc.ListCategories(ctx, req.GetIncludeInactive())
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &catalogv1.ListCategoriesResponse{}
	for _, c := range cats {
		out.Categories = append(out.Categories, toProtoCategory(c))
	}
	return out, nil
}

// ListProducts mengembalikan daftar produk.
func (s *Server) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
	products, total, page, limit, err := s.svc.ListProducts(ctx, service.ListProductsInput{
		CategorySlug:    req.GetCategorySlug(),
		Query:           req.GetQuery(),
		Sort:            req.GetSort(),
		Page:            int(req.GetPage()),
		Limit:           int(req.GetLimit()),
		IncludeInactive: req.GetIncludeInactive(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &catalogv1.ListProductsResponse{Page: int32(page), Limit: int32(limit), Total: total}
	for _, p := range products {
		out.Products = append(out.Products, toProtoProduct(p))
	}
	return out, nil
}

// GetProduct mengambil satu produk.
func (s *Server) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.Product, error) {
	p, err := s.svc.GetProduct(ctx, req.GetId(), req.GetSlug(), req.GetIncludeInactive())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoProduct(p), nil
}

// CreateProduct membuat produk baru.
func (s *Server) CreateProduct(ctx context.Context, req *catalogv1.CreateProductRequest) (*catalogv1.Product, error) {
	p, err := s.svc.CreateProduct(ctx, service.CreateProductInput{
		Name:        req.GetName(),
		Slug:        req.GetSlug(),
		CategoryID:  req.GetCategoryId(),
		Description: req.GetDescription(),
		Unit:        req.GetUnit(),
		Price:       req.GetPrice(),
		Stock:       req.GetStock(),
		MinQty:      req.GetMinQty(),
		StepQty:     req.GetStepQty(),
		IsActive:    req.GetIsActive(),
		Images:      req.GetImages(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoProduct(p), nil
}

// UpdateProduct menerapkan patch produk.
func (s *Server) UpdateProduct(ctx context.Context, req *catalogv1.UpdateProductRequest) (*catalogv1.Product, error) {
	p, err := s.svc.UpdateProduct(ctx, service.UpdateProductInput{
		ID:            req.GetId(),
		Name:          req.Name,
		Slug:          req.Slug,
		CategoryID:    req.CategoryId,
		Description:   req.Description,
		Unit:          req.Unit,
		Price:         req.Price,
		MinQty:        req.MinQty,
		StepQty:       req.StepQty,
		IsActive:      req.IsActive,
		Images:        req.GetImages(),
		ReplaceImages: req.GetReplaceImages(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoProduct(p), nil
}

// SetStock mengubah stok produk.
func (s *Server) SetStock(ctx context.Context, req *catalogv1.SetStockRequest) (*catalogv1.Product, error) {
	p, err := s.svc.SetStock(ctx, req.GetProductId(), req.GetMode(), req.GetQty())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoProduct(p), nil
}

// DeleteProduct melakukan soft delete.
func (s *Server) DeleteProduct(ctx context.Context, req *catalogv1.DeleteProductRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteProduct(ctx, req.GetId()); err != nil {
		return nil, s.toStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// CheckStock melaporkan ketersediaan stok.
func (s *Server) CheckStock(ctx context.Context, req *catalogv1.CheckStockRequest) (*catalogv1.CheckStockResponse, error) {
	items, err := s.svc.CheckStock(ctx, toDomainItems(req.GetItems()))
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &catalogv1.CheckStockResponse{AllOk: true}
	for _, it := range items {
		out.Items = append(out.Items, toProtoStatus(it))
		if !it.OK {
			out.AllOk = false
		}
	}
	return out, nil
}

// ReserveStock mengurangi stok untuk sebuah order.
func (s *Server) ReserveStock(ctx context.Context, req *catalogv1.ReserveStockRequest) (*catalogv1.ReserveStockResponse, error) {
	insufficient, err := s.svc.ReserveStock(ctx, req.GetOrderId(), toDomainItems(req.GetItems()))
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &catalogv1.ReserveStockResponse{Reserved: len(insufficient) == 0}
	for _, it := range insufficient {
		out.Insufficient = append(out.Insufficient, toProtoStatus(it))
	}
	return out, nil
}

// ReleaseStock mengembalikan stok reservasi order.
func (s *Server) ReleaseStock(ctx context.Context, req *catalogv1.ReleaseStockRequest) (*emptypb.Empty, error) {
	if err := s.svc.ReleaseStock(ctx, req.GetOrderId()); err != nil {
		return nil, s.toStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func toDomainItems(items []*catalogv1.StockItem) []domain.StockItem {
	out := make([]domain.StockItem, 0, len(items))
	for _, it := range items {
		out = append(out, domain.StockItem{ProductID: it.GetProductId(), Qty: it.GetQty()})
	}
	return out
}

func toProtoCategory(c domain.Category) *catalogv1.Category {
	return &catalogv1.Category{
		Id:       c.ID,
		Name:     c.Name,
		Slug:     c.Slug,
		Sort:     c.Sort,
		IsActive: c.IsActive,
	}
}

func toProtoProduct(p domain.Product) *catalogv1.Product {
	out := &catalogv1.Product{
		Id:          p.ID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Unit:        p.Unit,
		Price:       p.Price,
		Stock:       p.Stock,
		MinQty:      p.MinQty,
		StepQty:     p.StepQty,
		IsActive:    p.IsActive,
		Images:      p.Images,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
	if p.Category != nil {
		out.Category = toProtoCategory(*p.Category)
	}
	return out
}

func toProtoStatus(st domain.StockStatus) *catalogv1.StockStatus {
	return &catalogv1.StockStatus{
		ProductId: st.ProductID,
		Name:      st.Name,
		Requested: st.Requested,
		Available: st.Available,
		Ok:        st.OK,
	}
}

func (s *Server) toStatus(err error) error {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		st := status.New(codes.InvalidArgument, "Data yang dikirim tidak valid.")
		br := &errdetails.BadRequest{}
		for _, fe := range ve.Errors {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       fe.Field,
				Description: fe.Message,
			})
		}
		if withDetails, detailErr := st.WithDetails(br); detailErr == nil {
			return withDetails.Err()
		}
		return st.Err()
	}

	switch {
	case errors.Is(err, domain.ErrProductNotFound), errors.Is(err, domain.ErrCategoryNotFound):
		return status.Error(codes.NotFound, "Data tidak ditemukan.")
	case errors.Is(err, domain.ErrSlugExists):
		return status.Error(codes.AlreadyExists, "Slug produk sudah dipakai.")
	case errors.Is(err, domain.ErrInsufficientStock):
		return status.Error(codes.Aborted, "Stok tidak cukup.")
	case errors.Is(err, domain.ErrStockNegative):
		return status.Error(codes.FailedPrecondition, "Stok tidak boleh negatif.")
	default:
		s.log.Error("catalog internal error", "error", err)
		return status.Error(codes.Internal, "Terjadi kesalahan internal.")
	}
}
