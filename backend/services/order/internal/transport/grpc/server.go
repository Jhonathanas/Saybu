// Package grpc mengimplementasikan proto order.v1.OrderService.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "saybu/gen/saybu/order/v1"
	"saybu/services/order/internal/domain"
)

// Service adalah usecase yang dibutuhkan transport.
type Service interface {
	CreateOrder(ctx context.Context, in domain.CreateInput, idempotencyKey string) (domain.Order, error)
	GetOrderByCode(ctx context.Context, code, phone, userID string) (domain.Order, error)
	ListMyOrders(ctx context.Context, userID string, page, limit int) ([]domain.Order, int64, int, int, error)
	CancelOrder(ctx context.Context, code, phone, userID, reason string) (domain.Order, error)
}

// Server membungkus usecase pesanan sebagai gRPC.
type Server struct {
	orderv1.UnimplementedOrderServiceServer
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

// Register mendaftarkan OrderService ke gRPC server.
func Register(srv *googlegrpc.Server, svc Service, log *slog.Logger) {
	orderv1.RegisterOrderServiceServer(srv, New(svc, log))
}

// CreateOrder menjalankan checkout.
func (s *Server) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
	in := domain.CreateInput{
		Customer: domain.Customer{
			Name:  req.GetCustomer().GetName(),
			Phone: req.GetCustomer().GetPhone(),
			Email: req.GetCustomer().GetEmail(),
			Note:  req.GetCustomer().GetNote(),
		},
		Fulfillment:   req.GetFulfillment(),
		ZoneID:        req.GetZoneId(),
		Address:       toDomainAddress(req.GetAddress()),
		PaymentMethod: req.GetPaymentMethod(),
		UserID:        req.GetUserId(),
	}
	for _, it := range req.GetItems() {
		in.Items = append(in.Items, domain.ItemInput{ProductID: it.GetProductId(), Qty: it.GetQty()})
	}

	o, err := s.svc.CreateOrder(ctx, in, req.GetIdempotencyKey())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoOrder(o), nil
}

// GetOrderByCode mengambil pesanan untuk pelacakan.
func (s *Server) GetOrderByCode(ctx context.Context, req *orderv1.GetOrderByCodeRequest) (*orderv1.Order, error) {
	o, err := s.svc.GetOrderByCode(ctx, req.GetCode(), req.GetPhone(), req.GetUserId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoOrder(o), nil
}

// ListMyOrders mengambil daftar pesanan milik user.
func (s *Server) ListMyOrders(ctx context.Context, req *orderv1.ListMyOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	orders, total, page, limit, err := s.svc.ListMyOrders(ctx, req.GetUserId(), int(req.GetPage()), int(req.GetLimit()))
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &orderv1.ListOrdersResponse{Page: int32(page), Limit: int32(limit), Total: total}
	for _, o := range orders {
		out.Orders = append(out.Orders, toProtoOrder(o))
	}
	return out, nil
}

// CancelOrder membatalkan pesanan.
func (s *Server) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.Order, error) {
	o, err := s.svc.CancelOrder(ctx, req.GetCode(), req.GetPhone(), req.GetUserId(), req.GetReason())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoOrder(o), nil
}

func toDomainAddress(a *orderv1.Address) domain.Address {
	if a == nil {
		return domain.Address{}
	}
	return domain.Address{
		Line:       a.GetLine(),
		RTRW:       a.GetRtRw(),
		Kelurahan:  a.GetKelurahan(),
		Kecamatan:  a.GetKecamatan(),
		City:       a.GetCity(),
		PostalCode: a.GetPostalCode(),
		Landmark:   a.GetLandmark(),
	}
}

func toProtoOrder(o domain.Order) *orderv1.Order {
	out := &orderv1.Order{
		Id:            o.ID,
		Code:          o.Code,
		Status:        o.Status,
		Fulfillment:   o.Fulfillment,
		PaymentMethod: o.PaymentMethod,
		Customer: &orderv1.Customer{
			Name:  o.Customer.Name,
			Phone: o.Customer.Phone,
			Email: o.Customer.Email,
			Note:  o.Customer.Note,
		},
		Address: &orderv1.Address{
			Line:       o.Address.Line,
			RtRw:       o.Address.RTRW,
			Kelurahan:  o.Address.Kelurahan,
			Kecamatan:  o.Address.Kecamatan,
			City:       o.Address.City,
			PostalCode: o.Address.PostalCode,
			Landmark:   o.Address.Landmark,
		},
		ZoneId:       o.ZoneID,
		ZoneName:     o.ZoneName,
		Subtotal:     o.Subtotal,
		ShippingFee:  o.ShippingFee,
		Total:        o.Total,
		CancelReason: o.CancelReason,
		ExpiresAt:    o.ExpiresAt.Format(time.RFC3339),
		CreatedAt:    o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    o.UpdatedAt.Format(time.RFC3339),
	}
	for _, it := range o.Items {
		out.Items = append(out.Items, &orderv1.OrderItem{
			ProductId:  it.ProductID,
			Name:       it.Name,
			Unit:       it.Unit,
			Qty:        it.Qty,
			QtyDisplay: domain.DisplayQty(it.Unit, it.Qty),
			UnitPrice:  it.UnitPrice,
			LineTotal:  it.LineTotal,
		})
	}
	for _, h := range o.History {
		out.StatusHistory = append(out.StatusHistory, &orderv1.StatusHistory{
			Status: h.Status,
			Note:   h.Note,
			At:     h.At.Format(time.RFC3339),
		})
	}
	return out
}

func (s *Server) toStatus(err error) error {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		st := status.New(codes.InvalidArgument, "Data yang dikirim tidak valid.")
		if withDetails, detailErr := st.WithDetails(badRequest(ve.Errors)); detailErr == nil {
			return withDetails.Err()
		}
		return st.Err()
	}

	var ise *domain.InsufficientStockError
	if errors.As(err, &ise) {
		fields := make([]domain.FieldError, 0, len(ise.Items))
		for _, it := range ise.Items {
			name := it.Name
			if name == "" {
				name = it.ProductID
			}
			fields = append(fields, domain.FieldError{
				Field:   "items",
				Message: name + ": diminta " + formatQty(it.Requested) + ", tersedia " + formatQty(it.Available),
			})
		}
		st := status.New(codes.Aborted, "Stok tidak cukup untuk sebagian produk.")
		if withDetails, detailErr := st.WithDetails(badRequest(fields)); detailErr == nil {
			return withDetails.Err()
		}
		return st.Err()
	}

	var znf *domain.ZoneNotFoundError
	if errors.As(err, &znf) {
		st := status.New(codes.FailedPrecondition, "Zona ongkir tidak ditemukan.")
		if withDetails, detailErr := st.WithDetails(badRequest([]domain.FieldError{
			{Field: "zone_id", Message: "Zona ongkir tidak ditemukan."},
		})); detailErr == nil {
			return withDetails.Err()
		}
		return st.Err()
	}

	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		return status.Error(codes.NotFound, "Pesanan tidak ditemukan.")
	case errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrCancelNotAllowed):
		return status.Error(codes.Aborted, "Status pesanan tidak mengizinkan tindakan ini.")
	case errors.Is(err, domain.ErrIdempotencyMismatch):
		return status.Error(codes.FailedPrecondition, "Idempotency-Key sudah dipakai dengan isi berbeda.")
	case errors.Is(err, domain.ErrIdempotencyKeyMissing):
		return status.Error(codes.InvalidArgument, "Header Idempotency-Key wajib diisi.")
	case errors.Is(err, domain.ErrProductUnavailable):
		return status.Error(codes.FailedPrecondition, "Produk tidak tersedia.")
	default:
		s.log.Error("order internal error", "error", err)
		return status.Error(codes.Internal, "Terjadi kesalahan internal.")
	}
}

func badRequest(fields []domain.FieldError) *errdetails.BadRequest {
	br := &errdetails.BadRequest{}
	for _, fe := range fields {
		br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       fe.Field,
			Description: fe.Message,
		})
	}
	return br
}

// formatQty menampilkan qty dalam satuan terkecil (tanpa satuan).
func formatQty(v int64) string { return strconv.FormatInt(v, 10) }
