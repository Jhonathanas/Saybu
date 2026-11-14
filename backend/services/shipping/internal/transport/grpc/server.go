// Package grpc mengimplementasikan proto shipping.v1.ShippingService.
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

	shippingv1 "saybu/gen/saybu/shipping/v1"
	"saybu/services/shipping/internal/domain"
	"saybu/services/shipping/internal/service"
)

// Service adalah usecase yang dibutuhkan transport.
type Service interface {
	ListZones(ctx context.Context, includeInactive bool) ([]domain.Zone, error)
	Quote(ctx context.Context, zoneID string, subtotal int64) (domain.Quote, error)
	UpsertZone(ctx context.Context, in service.UpsertZoneInput) (domain.Zone, error)
	DeleteZone(ctx context.Context, id string) error
}

// Server membungkus usecase ongkir sebagai gRPC.
type Server struct {
	shippingv1.UnimplementedShippingServiceServer
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

// Register mendaftarkan ShippingService ke gRPC server.
func Register(srv *googlegrpc.Server, svc Service, log *slog.Logger) {
	shippingv1.RegisterShippingServiceServer(srv, New(svc, log))
}

// ListZones mengembalikan daftar zona ongkir.
func (s *Server) ListZones(ctx context.Context, req *shippingv1.ListZonesRequest) (*shippingv1.ListZonesResponse, error) {
	zones, err := s.svc.ListZones(ctx, req.GetIncludeInactive())
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &shippingv1.ListZonesResponse{}
	for _, z := range zones {
		out.Zones = append(out.Zones, toProtoZone(z))
	}
	return out, nil
}

// Quote menghitung ongkir untuk sebuah zona dan subtotal.
func (s *Server) Quote(ctx context.Context, req *shippingv1.QuoteRequest) (*shippingv1.QuoteResponse, error) {
	q, err := s.svc.Quote(ctx, req.GetZoneId(), req.GetSubtotal())
	if err != nil {
		// API_SPEC §4.2: zona tak dikenal pada kuotasi → 422 (bukan 404).
		if errors.Is(err, domain.ErrZoneNotFound) {
			return nil, status.Error(codes.FailedPrecondition, "Zona tidak dikenal.")
		}
		return nil, s.toStatus(err)
	}
	return &shippingv1.QuoteResponse{
		ZoneId:       q.ZoneID,
		ZoneName:     q.ZoneName,
		Fee:          q.Fee,
		FreeShipping: q.FreeShipping,
		Subtotal:     q.Subtotal,
		Total:        q.Total,
	}, nil
}

// UpsertZone membuat atau memperbarui zona.
func (s *Server) UpsertZone(ctx context.Context, req *shippingv1.UpsertZoneRequest) (*shippingv1.Zone, error) {
	z, err := s.svc.UpsertZone(ctx, service.UpsertZoneInput{
		ID:              req.GetId(),
		Name:            req.GetName(),
		Areas:           req.GetAreas(),
		Fee:             req.GetFee(),
		FreeMinSubtotal: req.GetFreeMinSubtotal(),
		IsActive:        req.IsActive,
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoZone(z), nil
}

// DeleteZone menghapus zona.
func (s *Server) DeleteZone(ctx context.Context, req *shippingv1.DeleteZoneRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteZone(ctx, req.GetId()); err != nil {
		return nil, s.toStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func toProtoZone(z domain.Zone) *shippingv1.Zone {
	return &shippingv1.Zone{
		Id:              z.ID,
		Name:            z.Name,
		Areas:           z.Areas,
		Fee:             z.Fee,
		FreeMinSubtotal: z.FreeMinSubtotal,
		IsActive:        z.IsActive,
		CreatedAt:       z.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       z.UpdatedAt.Format(time.RFC3339),
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
	case errors.Is(err, domain.ErrZoneNotFound):
		return status.Error(codes.NotFound, "Zona tidak ditemukan.")
	case errors.Is(err, domain.ErrZoneExists):
		return status.Error(codes.AlreadyExists, "Nama zona sudah dipakai.")
	default:
		s.log.Error("shipping internal error", "error", err)
		return status.Error(codes.Internal, "Terjadi kesalahan internal.")
	}
}
