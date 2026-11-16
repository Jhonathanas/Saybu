package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	orderv1 "saybu/gen/saybu/order/v1"
	"saybu/services/gateway/internal/authmw"
)

type fakeAdminOrder struct {
	getByID func(context.Context, *orderv1.GetOrderByIDRequest) (*orderv1.Order, error)
	list    func(context.Context, *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	update  func(context.Context, *orderv1.UpdateStatusRequest) (*orderv1.Order, error)
	stats   func(context.Context, *orderv1.GetTodayStatsRequest) (*orderv1.TodayStats, error)
}

func (f *fakeAdminOrder) GetOrderByID(ctx context.Context, in *orderv1.GetOrderByIDRequest, _ ...grpc.CallOption) (*orderv1.Order, error) {
	return f.getByID(ctx, in)
}
func (f *fakeAdminOrder) ListOrders(ctx context.Context, in *orderv1.ListOrdersRequest, _ ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
	return f.list(ctx, in)
}
func (f *fakeAdminOrder) UpdateStatus(ctx context.Context, in *orderv1.UpdateStatusRequest, _ ...grpc.CallOption) (*orderv1.Order, error) {
	return f.update(ctx, in)
}
func (f *fakeAdminOrder) GetTodayStats(ctx context.Context, in *orderv1.GetTodayStatsRequest, _ ...grpc.CallOption) (*orderv1.TodayStats, error) {
	return f.stats(ctx, in)
}

type fakeLowStock struct {
	list func(context.Context, *catalogv1.ListLowStockRequest) (*catalogv1.ListLowStockResponse, error)
}

func (f *fakeLowStock) ListLowStock(ctx context.Context, in *catalogv1.ListLowStockRequest, _ ...grpc.CallOption) (*catalogv1.ListLowStockResponse, error) {
	return f.list(ctx, in)
}

func newAdminHandler(orders AdminOrderService, catalog LowStockService) *Admin {
	return NewAdmin(orders, catalog, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestAdminListOrders(t *testing.T) {
	var got *orderv1.ListOrdersRequest
	h := newAdminHandler(&fakeAdminOrder{
		list: func(_ context.Context, in *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			got = in
			return &orderv1.ListOrdersResponse{Orders: []*orderv1.Order{sampleProtoOrder()}, Page: 1, Limit: 20, Total: 1}, nil
		},
	}, nil)

	rec := httptest.NewRecorder()
	h.ListOrders(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?status=PAID&q=budi&page=2&limit=10", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetStatus() != "PAID" || got.GetQ() != "budi" || got.GetPage() != 2 || got.GetLimit() != 10 {
		t.Fatalf("request = %+v", got)
	}
	body := decodeBody(t, rec)
	meta, _ := body["meta"].(map[string]any)
	if meta["total"] != float64(1) {
		t.Fatalf("meta = %+v", meta)
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data = %+v", data)
	}
}

func TestAdminGetOrder(t *testing.T) {
	var got *orderv1.GetOrderByIDRequest
	h := newAdminHandler(&fakeAdminOrder{
		getByID: func(_ context.Context, in *orderv1.GetOrderByIDRequest) (*orderv1.Order, error) {
			got = in
			return sampleProtoOrder(), nil
		},
	}, nil)

	req := withParam(httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/o-1", nil), "id", "o-1")
	rec := httptest.NewRecorder()
	h.GetOrder(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetId() != "o-1" {
		t.Fatalf("request = %+v", got)
	}
}

func TestAdminUpdateStatusForwardsActor(t *testing.T) {
	var got *orderv1.UpdateStatusRequest
	h := newAdminHandler(&fakeAdminOrder{
		update: func(_ context.Context, in *orderv1.UpdateStatusRequest) (*orderv1.Order, error) {
			got = in
			o := sampleProtoOrder()
			o.Status = "PREPARING"
			return o, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/o-1/status",
		strings.NewReader(`{"status":"PREPARING","note":"disiapkan"}`))
	req = withParam(req, "id", "o-1")
	req = req.WithContext(authmw.WithIdentity(req.Context(), authmw.Identity{UserID: "u-admin", Role: "admin"}))
	rec := httptest.NewRecorder()
	h.UpdateStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetId() != "o-1" || got.GetStatus() != "PREPARING" || got.GetNote() != "disiapkan" || got.GetActor() != "u-admin" {
		t.Fatalf("request = %+v", got)
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	if data["status"] != "PREPARING" {
		t.Fatalf("data = %+v", data)
	}
}

func TestAdminUpdateStatusIllegalTransitionMaps409(t *testing.T) {
	h := newAdminHandler(&fakeAdminOrder{
		update: func(context.Context, *orderv1.UpdateStatusRequest) (*orderv1.Order, error) {
			return nil, status.Error(codes.Aborted, "Transisi tidak sah.")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/o-1/status", strings.NewReader(`{"status":"PREPARING"}`))
	req = withParam(req, "id", "o-1")
	rec := httptest.NewRecorder()
	h.UpdateStatus(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestAdminTodayStatsMergesLowStock(t *testing.T) {
	h := newAdminHandler(&fakeAdminOrder{
		stats: func(context.Context, *orderv1.GetTodayStatsRequest) (*orderv1.TodayStats, error) {
			return &orderv1.TodayStats{OrdersCount: 12, RevenuePaid: 845000, AwaitingVerification: 3}, nil
		},
	}, &fakeLowStock{
		list: func(_ context.Context, in *catalogv1.ListLowStockRequest) (*catalogv1.ListLowStockResponse, error) {
			if in.GetLimit() != 10 {
				t.Fatalf("limit = %d", in.GetLimit())
			}
			return &catalogv1.ListLowStockResponse{Products: []*catalogv1.LowStockProduct{
				{ProductId: "p1", Name: "Tomat", Stock: 3000, Unit: "kg"},
			}}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.TodayStats(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/today", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	if data["orders_count"] != float64(12) || data["revenue_paid"] != float64(845000) || data["awaiting_verification"] != float64(3) {
		t.Fatalf("data = %+v", data)
	}
	low, _ := data["low_stock"].([]any)
	if len(low) != 1 {
		t.Fatalf("low_stock = %+v", data["low_stock"])
	}
	if p, _ := low[0].(map[string]any); p["product_id"] != "p1" || p["name"] != "Tomat" || p["unit"] != "kg" {
		t.Fatalf("low = %+v", low[0])
	}
}

func TestAdminTodayStatsLowStockFailureIsTolerated(t *testing.T) {
	h := newAdminHandler(&fakeAdminOrder{
		stats: func(context.Context, *orderv1.GetTodayStatsRequest) (*orderv1.TodayStats, error) {
			return &orderv1.TodayStats{OrdersCount: 1}, nil
		},
	}, &fakeLowStock{
		list: func(context.Context, *catalogv1.ListLowStockRequest) (*catalogv1.ListLowStockResponse, error) {
			return nil, status.Error(codes.Unavailable, "catalog down")
		},
	})

	rec := httptest.NewRecorder()
	h.TodayStats(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats/today", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	if data["orders_count"] != float64(1) {
		t.Fatalf("data = %+v", data)
	}
	if low, _ := data["low_stock"].([]any); len(low) != 0 {
		t.Fatalf("low_stock harus kosong: %+v", low)
	}
}
