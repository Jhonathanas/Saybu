package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "saybu/gen/saybu/order/v1"
	"saybu/services/gateway/internal/authmw"
	"saybu/services/gateway/internal/ratelimit"
)

type fakeOrder struct {
	create func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.Order, error)
	get    func(context.Context, *orderv1.GetOrderByCodeRequest) (*orderv1.Order, error)
	list   func(context.Context, *orderv1.ListMyOrdersRequest) (*orderv1.ListOrdersResponse, error)
	cancel func(context.Context, *orderv1.CancelOrderRequest) (*orderv1.Order, error)
}

func (f *fakeOrder) CreateOrder(ctx context.Context, in *orderv1.CreateOrderRequest, _ ...grpc.CallOption) (*orderv1.Order, error) {
	return f.create(ctx, in)
}
func (f *fakeOrder) GetOrderByCode(ctx context.Context, in *orderv1.GetOrderByCodeRequest, _ ...grpc.CallOption) (*orderv1.Order, error) {
	return f.get(ctx, in)
}
func (f *fakeOrder) ListMyOrders(ctx context.Context, in *orderv1.ListMyOrdersRequest, _ ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
	return f.list(ctx, in)
}
func (f *fakeOrder) CancelOrder(ctx context.Context, in *orderv1.CancelOrderRequest, _ ...grpc.CallOption) (*orderv1.Order, error) {
	return f.cancel(ctx, in)
}

func newOrderHandler(client OrderService) *Order {
	return NewOrder(client, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func sampleProtoOrder() *orderv1.Order {
	return &orderv1.Order{
		Id: "o-1", Code: "SB-20261114-0001", Status: "PENDING_PAYMENT",
		Fulfillment: "DELIVERY", PaymentMethod: "TRANSFER",
		Customer: &orderv1.Customer{Name: "Siti", Phone: "081234567890"},
		Address:  &orderv1.Address{Line: "Jl. Melati 12"},
		ZoneId:   "z-1", ZoneName: "Kec. Sukamaju",
		Subtotal: 25000, ShippingFee: 10000, Total: 35000,
		Items: []*orderv1.OrderItem{
			{ProductId: "p1", Name: "Bayam", Unit: "ikat", Qty: 2, QtyDisplay: "2 ikat", UnitPrice: 5000, LineTotal: 10000},
		},
		StatusHistory: []*orderv1.StatusHistory{{Status: "PENDING_PAYMENT", At: "2026-11-14T03:00:00Z"}},
		ExpiresAt:     "2026-11-15T03:00:00Z", CreatedAt: "2026-11-14T03:00:00Z",
	}
}

func TestCreateOrderMissingIdempotencyKey(t *testing.T) {
	h := newOrderHandler(&fakeOrder{})

	rec := httptest.NewRecorder()
	h.CreateOrder(rec, httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateOrderSuccess(t *testing.T) {
	var got *orderv1.CreateOrderRequest
	h := newOrderHandler(&fakeOrder{
		create: func(_ context.Context, in *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
			got = in
			return sampleProtoOrder(), nil
		},
	})

	payload := `{
		"customer": {"name": "Siti", "phone": "081234567890", "note": "segar"},
		"fulfillment": "DELIVERY",
		"zone_id": "z-1",
		"address": {"line": "Jl. Melati 12", "rt_rw": "003/004"},
		"payment_method": "TRANSFER",
		"items": [{"product_id": "p1", "qty": 2}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(payload))
	req.Header.Set("Idempotency-Key", "key-1")
	rec := httptest.NewRecorder()
	h.CreateOrder(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if got.GetIdempotencyKey() != "key-1" || got.GetItems()[0].GetQty() != 2 {
		t.Fatalf("request = %+v", got)
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	if data["code"] != "SB-20261114-0001" || data["total"] != float64(35000) {
		t.Fatalf("data = %+v", data)
	}
	if data["zone_name"] != "Kec. Sukamaju" {
		t.Fatalf("data = %+v", data)
	}
}

func TestCreateOrderForwardsIdentity(t *testing.T) {
	var got *orderv1.CreateOrderRequest
	h := newOrderHandler(&fakeOrder{
		create: func(_ context.Context, in *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
			got = in
			return sampleProtoOrder(), nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders",
		strings.NewReader(`{"customer":{"name":"S","phone":"0812345678"},"fulfillment":"PICKUP","payment_method":"COD","items":[{"product_id":"p1","qty":1}]}`))
	req.Header.Set("Idempotency-Key", "key-2")
	req = req.WithContext(authmw.WithIdentity(req.Context(), authmw.Identity{UserID: "u-9", Role: "customer"}))
	rec := httptest.NewRecorder()
	h.CreateOrder(rec, req)

	if got.GetUserId() != "u-9" {
		t.Fatalf("user_id tidak diteruskan: %+v", got)
	}
}

func TestCreateOrderStockConflictMaps409(t *testing.T) {
	h := newOrderHandler(&fakeOrder{
		create: func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
			return nil, status.Error(codes.Aborted, "Stok tidak cukup.")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "key-3")
	rec := httptest.NewRecorder()
	h.CreateOrder(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestCreateOrderZoneNotFoundMaps422WithDetails(t *testing.T) {
	st := status.New(codes.FailedPrecondition, "Zona ongkir tidak ditemukan.")
	st, _ = st.WithDetails(&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{
		{Field: "zone_id", Description: "Zona ongkir tidak ditemukan."},
	}})
	h := newOrderHandler(&fakeOrder{
		create: func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
			return nil, st.Err()
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "key-zone")
	rec := httptest.NewRecorder()
	h.CreateOrder(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	body := decodeBody(t, rec)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "BUSINESS_RULE" {
		t.Fatalf("code = %+v", errObj)
	}
	details, _ := errObj["details"].([]any)
	if len(details) != 1 {
		t.Fatalf("details = %+v", errObj["details"])
	}
	if d, _ := details[0].(map[string]any); d["field"] != "zone_id" {
		t.Fatalf("detail = %+v", details[0])
	}
}

func TestCreateOrderIdempotencyMismatchMaps422(t *testing.T) {
	h := newOrderHandler(&fakeOrder{
		create: func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
			return nil, status.Error(codes.FailedPrecondition, "Idempotency-Key dipakai dengan isi berbeda.")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "key-4")
	rec := httptest.NewRecorder()
	h.CreateOrder(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestCreateOrderRateLimited(t *testing.T) {
	limiter := ratelimit.New(1, time.Minute)
	h := NewOrder(&fakeOrder{
		create: func(context.Context, *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
			return sampleProtoOrder(), nil
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), limiter)

	newReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{}`))
		r.Header.Set("Idempotency-Key", "k")
		return r
	}

	rec1 := httptest.NewRecorder()
	h.CreateOrder(rec1, newReq())
	if rec1.Code != http.StatusCreated {
		t.Fatalf("pertama status = %d, want 201", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.CreateOrder(rec2, newReq())
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("kedua status = %d, want 429", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After harus diisi")
	}
}

func TestGetOrder(t *testing.T) {
	var got *orderv1.GetOrderByCodeRequest
	h := newOrderHandler(&fakeOrder{
		get: func(_ context.Context, in *orderv1.GetOrderByCodeRequest) (*orderv1.Order, error) {
			got = in
			return sampleProtoOrder(), nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodGet, "/api/v1/orders/SB-20261114-0001?phone=081234567890", nil), "code", "SB-20261114-0001")
	h.GetOrder(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetCode() != "SB-20261114-0001" || got.GetPhone() != "081234567890" {
		t.Fatalf("request = %+v", got)
	}
}

func TestListMyOrdersRequiresAuth(t *testing.T) {
	h := newOrderHandler(&fakeOrder{})

	rec := httptest.NewRecorder()
	h.ListMyOrders(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me/orders", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestListMyOrders(t *testing.T) {
	var got *orderv1.ListMyOrdersRequest
	h := newOrderHandler(&fakeOrder{
		list: func(_ context.Context, in *orderv1.ListMyOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			got = in
			return &orderv1.ListOrdersResponse{Orders: []*orderv1.Order{sampleProtoOrder()}, Page: 1, Limit: 20, Total: 1}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/orders?page=1&limit=20", nil)
	req = req.WithContext(authmw.WithIdentity(req.Context(), authmw.Identity{UserID: "u-1", Role: "customer"}))
	rec := httptest.NewRecorder()
	h.ListMyOrders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetUserId() != "u-1" {
		t.Fatalf("request = %+v", got)
	}
	body := decodeBody(t, rec)
	meta, _ := body["meta"].(map[string]any)
	if meta["total"] != float64(1) {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestCancelOrder(t *testing.T) {
	var got *orderv1.CancelOrderRequest
	h := newOrderHandler(&fakeOrder{
		cancel: func(_ context.Context, in *orderv1.CancelOrderRequest) (*orderv1.Order, error) {
			got = in
			o := sampleProtoOrder()
			o.Status = "CANCELLED"
			return o, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/SB-1/cancel", strings.NewReader(`{"phone":"081234567890","reason":"berubah pikiran"}`))
	req = withParam(req, "code", "SB-1")
	rec := httptest.NewRecorder()
	h.CancelOrder(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetCode() != "SB-1" || got.GetReason() != "berubah pikiran" {
		t.Fatalf("request = %+v", got)
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	if data["status"] != "CANCELLED" {
		t.Fatalf("data = %+v", data)
	}
}

func TestCancelOrderConflictMaps409(t *testing.T) {
	h := newOrderHandler(&fakeOrder{
		cancel: func(context.Context, *orderv1.CancelOrderRequest) (*orderv1.Order, error) {
			return nil, status.Error(codes.Aborted, "Status tidak mengizinkan.")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/SB-1/cancel", strings.NewReader(`{"phone":"0812"}`))
	req = withParam(req, "code", "SB-1")
	rec := httptest.NewRecorder()
	h.CancelOrder(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestOrderRejectsUnknownFields(t *testing.T) {
	h := newOrderHandler(&fakeOrder{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{"customer":{"name":"X"},"tidak_dikenal":1}`))
	req.Header.Set("Idempotency-Key", "k")
	rec := httptest.NewRecorder()
	h.CreateOrder(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
