// Package grpc mengimplementasikan proto auth.v1.AuthService.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "saybu/gen/saybu/auth/v1"
	"saybu/services/auth/internal/domain"
	"saybu/services/auth/internal/service"
)

// Service adalah usecase yang dibutuhkan transport.
type Service interface {
	Register(ctx context.Context, in service.RegisterInput) (service.AuthResult, error)
	Login(ctx context.Context, identifier, password, userAgent, ip string) (service.AuthResult, error)
	Refresh(ctx context.Context, rawToken, userAgent, ip string) (service.AuthResult, error)
	Logout(ctx context.Context, rawToken string) error
	GetUser(ctx context.Context, id string) (domain.User, error)
	ListUsers(ctx context.Context, page, limit int, query string) ([]domain.User, int64, int, error)
}

// Server membungkus usecase auth sebagai gRPC.
type Server struct {
	authv1.UnimplementedAuthServiceServer
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

// Register mendaftarkan AuthService ke gRPC server.
func Register(srv *googlegrpc.Server, svc Service, log *slog.Logger) {
	authv1.RegisterAuthServiceServer(srv, New(svc, log))
}

// Register mendaftarkan user baru.
func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.AuthResponse, error) {
	ua, ip := clientInfo(ctx)
	res, err := s.svc.Register(ctx, service.RegisterInput{
		Name:      req.GetName(),
		Phone:     req.GetPhone(),
		Email:     req.GetEmail(),
		Password:  req.GetPassword(),
		UserAgent: ua,
		IP:        ip,
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoAuth(res), nil
}

// Login memverifikasi kredensial.
func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.AuthResponse, error) {
	ua, ip := clientInfo(ctx)
	res, err := s.svc.Login(ctx, req.GetIdentifier(), req.GetPassword(), ua, ip)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoAuth(res), nil
}

// Refresh merotasi refresh token.
func (s *Server) Refresh(ctx context.Context, req *authv1.RefreshRequest) (*authv1.AuthResponse, error) {
	ua, ip := clientInfo(ctx)
	res, err := s.svc.Refresh(ctx, req.GetRefreshToken(), ua, ip)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoAuth(res), nil
}

// Logout mencabut refresh token.
func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*emptypb.Empty, error) {
	if err := s.svc.Logout(ctx, req.GetRefreshToken()); err != nil {
		return nil, s.toStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// GetUser mengambil profil user.
func (s *Server) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.User, error) {
	u, err := s.svc.GetUser(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toProtoUser(u), nil
}

// ListUsers mengambil daftar user (admin).
func (s *Server) ListUsers(ctx context.Context, req *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
	users, total, page, err := s.svc.ListUsers(ctx, int(req.GetPage()), int(req.GetLimit()), req.GetQuery())
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := &authv1.ListUsersResponse{
		Page:  int32(page),
		Limit: int32(limitOrDefault(req.GetLimit())),
		Total: total,
	}
	for _, u := range users {
		out.Users = append(out.Users, toProtoUser(u))
	}
	return out, nil
}

func limitOrDefault(limit int32) int {
	if limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return int(limit)
}

func toProtoUser(u domain.User) *authv1.User {
	return &authv1.User{
		Id:        u.ID,
		Name:      u.Name,
		Phone:     u.Phone,
		Email:     u.EmailString(),
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

func toProtoAuth(res service.AuthResult) *authv1.AuthResponse {
	return &authv1.AuthResponse{
		User:             toProtoUser(res.User),
		AccessToken:      res.AccessToken,
		ExpiresIn:        int64(res.AccessExpiresIn.Seconds()),
		RefreshToken:     res.RefreshToken,
		RefreshExpiresIn: int64(res.RefreshExpiresIn.Seconds()),
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
	case errors.Is(err, domain.ErrUserExists):
		return status.Error(codes.AlreadyExists, "Nomor HP atau email sudah terdaftar.")
	case errors.Is(err, domain.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, "Nomor HP/email atau password salah.")
	case errors.Is(err, domain.ErrTokenReused), errors.Is(err, domain.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, "Sesi tidak valid. Silakan masuk lagi.")
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, "User tidak ditemukan.")
	default:
		// Error tak terduga: catat detailnya untuk diagnosis, jangan bocorkan ke klien.
		s.log.Error("auth internal error", "error", err)
		return status.Error(codes.Internal, "Terjadi kesalahan internal.")
	}
}

func clientInfo(ctx context.Context) (userAgent, ip string) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ""
	}
	userAgent = first(md.Get("user-agent"))
	ip = first(md.Get("x-forwarded-for"))
	if ip == "" {
		ip = first(md.Get("x-real-ip"))
	}
	if idx := strings.IndexByte(ip, ','); idx >= 0 {
		ip = strings.TrimSpace(ip[:idx])
	}
	return userAgent, ip
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
