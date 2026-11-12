package grpcx

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func TestServerWithInterceptors(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(log, nil)

	hs := health.NewServer()
	hs.SetServingStatus("test", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)

	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("status = %v", resp.Status)
	}
}
