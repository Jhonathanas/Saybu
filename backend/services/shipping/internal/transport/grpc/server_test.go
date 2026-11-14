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

	shippingv1 "saybu/gen/saybu/shipping/v1"
	"saybu/services/shipping/internal/domain"
	"saybu/services/shipping/internal/service"
)

type stubService struct {
	zones     []domain.Zone
	listErr   error
	quote     domain.Quote
	quoteErr  error
	upserted  domain.Zone
	upsertErr error
	deleteErr error
}

func (s *stubService) ListZones(context.Context, bool) ([]domain.Zone, error) {
	return s.zones, s.listErr
}
func (s *stubService) Quote(context.Context, string, int64) (domain.Quote, error) {
	return s.quote, s.quoteErr
}
func (s *stubService) UpsertZone(context.Context, service.UpsertZoneInput) (domain.Zone, error) {
	return s.upserted, s.upsertErr
}
func (s *stubService) DeleteZone(context.Context, string) error { return s.deleteErr }

func startServer(t *testing.T, svc Service) shippingv1.ShippingServiceClient {
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
	return shippingv1.NewShippingServiceClient(conn)
}

func TestListZones(t *testing.T) {
	created := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	stub := &stubService{zones: []domain.Zone{{
		ID: "z1", Name: "Kec. Sukamaju", Areas: []string{"Sukamaju"},
		Fee: 10000, FreeMinSubtotal: 150000, IsActive: true,
		CreatedAt: created, UpdatedAt: created,
	}}}
	client := startServer(t, stub)

	resp, err := client.ListZones(context.Background(), &shippingv1.ListZonesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetZones()) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	z := resp.GetZones()[0]
	if z.GetName() != "Kec. Sukamaju" || z.GetFee() != 10000 || len(z.GetAreas()) != 1 {
		t.Fatalf("zone = %+v", z)
	}
}

func TestQuote(t *testing.T) {
	stub := &stubService{quote: domain.Quote{
		ZoneID: "z1", ZoneName: "Kec. Sukamaju", Fee: 10000,
		Subtotal: 75000, Total: 85000,
	}}
	client := startServer(t, stub)

	resp, err := client.Quote(context.Background(), &shippingv1.QuoteRequest{ZoneId: "z1", Subtotal: 75000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetFee() != 10000 || resp.GetTotal() != 85000 || resp.GetZoneName() != "Kec. Sukamaju" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestQuoteUnknownZoneMaps422(t *testing.T) {
	stub := &stubService{quoteErr: domain.ErrZoneNotFound}
	client := startServer(t, stub)

	_, err := client.Quote(context.Background(), &shippingv1.QuoteRequest{ZoneId: "x", Subtotal: 1})
	if st, _ := status.FromError(err); st.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %v", st.Code())
	}
}

func TestUpsertZoneValidation(t *testing.T) {
	stub := &stubService{upsertErr: &domain.ValidationError{Errors: []domain.FieldError{
		{Field: "fee", Message: "Ongkir tidak boleh negatif."},
	}}}
	client := startServer(t, stub)

	_, err := client.UpsertZone(context.Background(), &shippingv1.UpsertZoneRequest{Name: "X"})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
	}
	found := false
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				if v.GetField() == "fee" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("detail fee tidak ada: %+v", st.Details())
	}
}

func TestUpsertZoneDuplicate(t *testing.T) {
	stub := &stubService{upsertErr: domain.ErrZoneExists}
	client := startServer(t, stub)

	_, err := client.UpsertZone(context.Background(), &shippingv1.UpsertZoneRequest{Name: "Kec. Sukamaju", Fee: 1})
	if st, _ := status.FromError(err); st.Code() != codes.AlreadyExists {
		t.Fatalf("code = %v", st.Code())
	}
}

func TestDeleteZoneNotFound(t *testing.T) {
	stub := &stubService{deleteErr: domain.ErrZoneNotFound}
	client := startServer(t, stub)

	_, err := client.DeleteZone(context.Background(), &shippingv1.DeleteZoneRequest{Id: "x"})
	if st, _ := status.FromError(err); st.Code() != codes.NotFound {
		t.Fatalf("code = %v", st.Code())
	}
}
