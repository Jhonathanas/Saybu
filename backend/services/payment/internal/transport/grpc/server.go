// Package grpc mengimplementasikan proto payment.v1.PaymentService.
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

	paymentv1 "saybu/gen/saybu/payment/v1"
	"saybu/services/payment/internal/domain"
)

// Service adalah usecase yang dibutuhkan transport.
type Service interface {
	GetByOrderCode(ctx context.Context, code, phone, userID string) (domain.Payment, error)
	UploadProof(ctx context.Context, in domain.UploadProofInput) (domain.Payment, error)
	List(ctx context.Context, status string, page, limit int) ([]domain.Payment, int64, int, int, error)
	Verify(ctx context.Context, id, decision, note, adminID string) (domain.Payment, error)
}

// Server membungkus usecase pembayaran sebagai gRPC.
type Server struct {
	paymentv1.UnimplementedPaymentServiceServer
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

// Register mendaftarkan PaymentService ke gRPC server.
func Register(srv *googlegrpc.Server, svc Service, log *slog.Logger) {
	paymentv1.RegisterPaymentServiceServer(srv, New(svc, log))
}

// GetPaymentByOrderCode mengambil status pembayaran sebuah pesanan.
func (s *Server) GetPaymentByOrderCode(ctx context.Context, req *paymentv1.GetPaymentByOrderCodeRequest) (*paymentv1.Payment, error) {
	p, err := s.svc.GetByOrderCode(ctx, req.GetCode(), req.GetPhone(), req.GetUserId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoPayment(p), nil
}

// UploadProof mendaftarkan bukti transfer.
func (s *Server) UploadProof(ctx context.Context, req *paymentv1.UploadProofRequest) (*paymentv1.Payment, error) {
	p, err := s.svc.UploadProof(ctx, domain.UploadProofInput{
		Code:        req.GetCode(),
		Phone:       req.GetPhone(),
		UserID:      req.GetUserId(),
		ProofURL:    req.GetProofUrl(),
		SizeBytes:   req.GetSizeBytes(),
		ContentType: req.GetContentType(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoPayment(p), nil
}

// ListPayments mengambil daftar pembayaran untuk panel admin.
func (s *Server) ListPayments(ctx context.Context, req *paymentv1.ListPaymentsRequest) (*paymentv1.ListPaymentsResponse, error) {
	payments, total, page, limit, err := s.svc.List(ctx, req.GetStatus(), int(req.GetPage()), int(req.GetLimit()))
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &paymentv1.ListPaymentsResponse{Page: int32(page), Limit: int32(limit), Total: total}
	for _, p := range payments {
		out.Payments = append(out.Payments, toProtoPayment(p))
	}
	return out, nil
}

// VerifyPayment menyetujui atau menolak bukti transfer (admin).
func (s *Server) VerifyPayment(ctx context.Context, req *paymentv1.VerifyPaymentRequest) (*paymentv1.Payment, error) {
	p, err := s.svc.Verify(ctx, req.GetId(), req.GetDecision(), req.GetNote(), req.GetAdminId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoPayment(p), nil
}

func toProtoPayment(p domain.Payment) *paymentv1.Payment {
	out := &paymentv1.Payment{
		Id:            p.ID,
		OrderId:       p.OrderID,
		OrderCode:     p.OrderCode,
		Method:        p.Method,
		Status:        p.Status,
		Amount:        p.Amount,
		CustomerName:  p.CustomerName,
		CustomerPhone: p.CustomerPhone,
		ProofUrl:      p.ProofURL,
		VerifiedBy:    p.VerifiedBy,
		Note:          p.Note,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt.Format(time.RFC3339),
	}
	if p.ProofUploadedAt != nil {
		out.ProofUploadedAt = p.ProofUploadedAt.Format(time.RFC3339)
	}
	if p.VerifiedAt != nil {
		out.VerifiedAt = p.VerifiedAt.Format(time.RFC3339)
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

	switch {
	case errors.Is(err, domain.ErrPaymentNotFound):
		return status.Error(codes.NotFound, "Pembayaran tidak ditemukan.")
	case errors.Is(err, domain.ErrProofNotAllowed), errors.Is(err, domain.ErrVerifyNotAllowed):
		return status.Error(codes.Aborted, "Status pembayaran tidak mengizinkan tindakan ini.")
	case errors.Is(err, domain.ErrInvalidDecision):
		return status.Error(codes.InvalidArgument, "Keputusan verifikasi tidak valid.")
	default:
		s.log.Error("payment internal error", "error", err)
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
