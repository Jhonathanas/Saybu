// Package grpcx menyediakan server/client gRPC dengan interceptor standar:
// request-id, logging, recovery, metrik, dan propagasi request-id ke downstream.
package grpcx

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"saybu/pkg/telemetry"
)

// NewServer membuat gRPC server dengan rantai interceptor standar.
func NewServer(log *slog.Logger, m *telemetry.Metrics) *grpc.Server {
	unary := []grpc.UnaryServerInterceptor{
		RequestIDUnary(),
		RecoveryUnary(log),
		LoggingUnary(log),
	}
	if m != nil {
		unary = append(unary, m.UnaryServerInterceptor())
	}
	return grpc.NewServer(
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(RequestIDStream(), RecoveryStream(log)),
	)
}

// RequestIDUnary mengambil X-Request-Id dari metadata atau membuat baru.
func RequestIDUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(withRequestID(ctx), req)
	}
}

// RequestIDStream sama seperti RequestIDUnary untuk stream.
func RequestIDStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: withRequestID(ss.Context())})
	}
}

func withRequestID(ctx context.Context) context.Context {
	id := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(telemetry.HeaderRequestID); len(vals) > 0 {
			id = vals[0]
		}
	}
	if id == "" {
		id = telemetry.NewRequestID()
	}
	_ = grpc.SetHeader(ctx, metadata.Pairs(telemetry.HeaderRequestID, id))
	return telemetry.WithRequestID(ctx, id)
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

// LoggingUnary mencatat method, durasi, dan kode status.
func LoggingUnary(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Info("grpc request",
			"request_id", telemetry.RequestID(ctx),
			"method", info.FullMethod,
			"code", status.Code(err).String(),
			"duration_ms", time.Since(start).Milliseconds())
		return resp, err
	}
}

// RecoveryUnary mengubah panic menjadi codes.Internal.
func RecoveryUnary(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("grpc panic recovered",
					"request_id", telemetry.RequestID(ctx),
					"method", info.FullMethod, "panic", rec)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// RecoveryStream mengubah panic pada stream menjadi codes.Internal.
func RecoveryStream(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("grpc stream panic recovered",
					"request_id", telemetry.RequestID(ss.Context()),
					"method", info.FullMethod, "panic", rec)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(srv, ss)
	}
}

// UnaryClientInterceptor meneruskan request-id ke service downstream.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id := telemetry.RequestID(ctx); id != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, telemetry.HeaderRequestID, id)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// Dial membuat koneksi gRPC internal (tanpa TLS) dengan propagasi request-id.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(UnaryClientInterceptor()),
	)
}
