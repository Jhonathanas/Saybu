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

	orderv1 "saybu/gen/saybu/order/v1"
	"saybu/services/order/internal/domain"
)

type stubService struct {
	create   func(context.Context, domain.CreateInput, string) (domain.Order, error)
	get      func(context.Context, string, string, string) (domain.Order, error)
	list     func(context.Context, string, int, int) ([]domain.Order, int64, int, int, error)
	cancel   func(context.Context, string, string, string, string) (domain.Order, error)
	lastIn   domain.CreateInput
	lastKey  string
	lastArgs []string
}

func (s *stubService) CreateOrder(ctx context.Context, in domain.CreateInput, key string) (domain.Order, error) {
	s.lastIn, s.lastKey = in, key
	return s.create(ctx, in, key)
}

func (s *stubService) GetOrderByCode(ctx context.Context, code, phone, userID string) (domain.Order, error) {
	s.lastArgs = []string{code, phone, userID}
	return s.get(ctx, code, phone, userID)
}

func (s *stubService) ListMyOrders(ctx context.Context, userID string, page, limit int) ([]domain.Order, int64, int, int, error) {
	return s.list(ctx, userID, page, limit)
}

func (s *stubService) CancelOrder(ctx context.Context, code, phone, userID, reason string) (domain.Order, error) {
	s.lastArgs = []string{code, phone, userID, reason}
	return s.cancel(ctx, code, phone, userID, reason)
}

func startServer(t *testing.T, svc Service) orderv1.OrderServiceClient {
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
	return orderv1.NewOrderServiceClient(conn)
}

func sampleOrder() domain.Order {
	created := time.Date(2026, 11, 14, 3, 0, 0, 0, time.UTC)
	return domain.Order{
		ID: "o-1", Code: "SB-20261114-0001", Status: domain.StatusPendingPayment,
		Fulfillment: domain.FulfillmentDelivery, PaymentMethod: domain.PaymentTransfer,
		Customer: domain.Customer{Name: "Siti", Phone: "081234567890"},
		Address:  domain.Address{Line: "Jl. Melati 12"},
		ZoneID:   "z-1", ZoneName: "Kec. Sukamaju",
		ShippingFee: 10000, Subtotal: 25000, Total: 35000,
		Items: []domain.Item{
			{ProductID: "p1", Name: "Bayam", Unit: "ikat", Qty: 2, UnitPrice: 5000, LineTotal: 10000},
			{ProductID: "p2", Name: "Tomat", Unit: "kg", Qty: 1000, UnitPrice: 15000, LineTotal: 15000},
		},
		History:   []domain.StatusEntry{{Status: domain.StatusPendingPayment, Actor: "system", At: created}},
		ExpiresAt: created.Add(24 * time.Hour),
		CreatedAt: created, UpdatedAt: created,
	}
}

func TestCreateOrderMapsInputAndOutput(t *testing.T) {
	stub := &stubService{create: func(context.Context, domain.CreateInput, string) (domain.Order, error) {
		return sampleOrder(), nil
	}}
	client := startServer(t, stub)

	resp, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{
		Customer:       &orderv1.Customer{Name: "Siti", Phone: "081234567890", Note: "segar"},
		Fulfillment:    "DELIVERY",
		ZoneId:         "z-1",
		Address:        &orderv1.Address{Line: "Jl. Melati 12", RtRw: "003/004"},
		PaymentMethod:  "TRANSFER",
		Items:          []*orderv1.OrderItemInput{{ProductId: "p1", Qty: 2}},
		UserId:         "u-1",
		IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.lastKey != "key-1" || stub.lastIn.UserID != "u-1" {
		t.Fatalf("input tidak dipetakan: key=%q user=%q", stub.lastKey, stub.lastIn.UserID)
	}
	if stub.lastIn.Address.RTRW != "003/004" || stub.lastIn.Items[0].ProductID != "p1" {
		t.Fatalf("input tidak dipetakan: %+v", stub.lastIn)
	}
	if resp.GetCode() != "SB-20261114-0001" || resp.GetTotal() != 35000 {
		t.Fatalf("resp = %+v", resp)
	}
	if len(resp.GetItems()) != 2 || resp.GetItems()[1].GetQtyDisplay() != "1 kg" {
		t.Fatalf("items = %+v", resp.GetItems())
	}
	if resp.GetZoneName() != "Kec. Sukamaju" || resp.GetExpiresAt() == "" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestCreateOrderValidationMaps400WithDetails(t *testing.T) {
	stub := &stubService{create: func(context.Context, domain.CreateInput, string) (domain.Order, error) {
		return domain.Order{}, (&domain.ValidationError{}).Add("customer.name", "Nama pembeli wajib diisi.")
	}}
	client := startServer(t, stub)

	_, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
	}
	found := false
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				if v.GetField() == "customer.name" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("detail customer.name tidak ada: %+v", st.Details())
	}
}

func TestCreateOrderInsufficientStockMapsAborted(t *testing.T) {
	stub := &stubService{create: func(context.Context, domain.CreateInput, string) (domain.Order, error) {
		return domain.Order{}, &domain.InsufficientStockError{Items: []domain.StockShort{
			{ProductID: "p1", Name: "Bayam", Requested: 5, Available: 2},
		}}
	}}
	client := startServer(t, stub)

	_, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{})
	if st, _ := status.FromError(err); st.Code() != codes.Aborted {
		t.Fatalf("code = %v, want Aborted", st.Code())
	}
}

func TestGetOrderNotFound(t *testing.T) {
	stub := &stubService{get: func(context.Context, string, string, string) (domain.Order, error) {
		return domain.Order{}, domain.ErrOrderNotFound
	}}
	client := startServer(t, stub)

	_, err := client.GetOrderByCode(context.Background(), &orderv1.GetOrderByCodeRequest{Code: "x", Phone: "0812"})
	if st, _ := status.FromError(err); st.Code() != codes.NotFound {
		t.Fatalf("code = %v", st.Code())
	}
}

func TestGetOrderForwardsArgs(t *testing.T) {
	stub := &stubService{get: func(context.Context, string, string, string) (domain.Order, error) {
		return sampleOrder(), nil
	}}
	client := startServer(t, stub)

	if _, err := client.GetOrderByCode(context.Background(), &orderv1.GetOrderByCodeRequest{
		Code: "SB-1", Phone: "0812", UserId: "u-1",
	}); err != nil {
		t.Fatal(err)
	}
	if stub.lastArgs[0] != "SB-1" || stub.lastArgs[1] != "0812" || stub.lastArgs[2] != "u-1" {
		t.Fatalf("args = %+v", stub.lastArgs)
	}
}

func TestCancelOrderNotAllowedMapsAborted(t *testing.T) {
	stub := &stubService{cancel: func(context.Context, string, string, string, string) (domain.Order, error) {
		return domain.Order{}, domain.ErrCancelNotAllowed
	}}
	client := startServer(t, stub)

	_, err := client.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{Code: "SB-1", Phone: "0812"})
	if st, _ := status.FromError(err); st.Code() != codes.Aborted {
		t.Fatalf("code = %v, want Aborted", st.Code())
	}
}

func TestCancelOrderInvalidTransitionMapsAborted(t *testing.T) {
	stub := &stubService{cancel: func(context.Context, string, string, string, string) (domain.Order, error) {
		return domain.Order{}, domain.ErrInvalidTransition
	}}
	client := startServer(t, stub)

	_, err := client.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{Code: "SB-1", Phone: "0812"})
	if st, _ := status.FromError(err); st.Code() != codes.Aborted {
		t.Fatalf("code = %v, want Aborted", st.Code())
	}
}

func TestCreateOrderZoneNotFoundMapsFailedPrecondition(t *testing.T) {
	stub := &stubService{create: func(context.Context, domain.CreateInput, string) (domain.Order, error) {
		return domain.Order{}, &domain.ZoneNotFoundError{ZoneID: "z-1"}
	}}
	client := startServer(t, stub)

	_, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{})
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", st.Code())
	}
	found := false
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				if v.GetField() == "zone_id" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("detail zone_id tidak ada: %+v", st.Details())
	}
}

func TestIdempotencyMismatchMapsFailedPrecondition(t *testing.T) {
	stub := &stubService{create: func(context.Context, domain.CreateInput, string) (domain.Order, error) {
		return domain.Order{}, domain.ErrIdempotencyMismatch
	}}
	client := startServer(t, stub)

	_, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{})
	if st, _ := status.FromError(err); st.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", st.Code())
	}
}

func TestListMyOrders(t *testing.T) {
	stub := &stubService{list: func(_ context.Context, userID string, page, limit int) ([]domain.Order, int64, int, int, error) {
		if userID != "u-1" || page != 2 || limit != 10 {
			t.Fatalf("args = %s %d %d", userID, page, limit)
		}
		return []domain.Order{sampleOrder()}, 1, page, limit, nil
	}}
	client := startServer(t, stub)

	resp, err := client.ListMyOrders(context.Background(), &orderv1.ListMyOrdersRequest{UserId: "u-1", Page: 2, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetTotal() != 1 || len(resp.GetOrders()) != 1 || resp.GetPage() != 2 {
		t.Fatalf("resp = %+v", resp)
	}
}
