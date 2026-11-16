package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"google.golang.org/grpc"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	orderv1 "saybu/gen/saybu/order/v1"
	"saybu/pkg/httpx"
)

// AdminOrderService adalah bagian klien gRPC order untuk panel admin.
type AdminOrderService interface {
	GetOrderByID(ctx context.Context, in *orderv1.GetOrderByIDRequest, opts ...grpc.CallOption) (*orderv1.Order, error)
	ListOrders(ctx context.Context, in *orderv1.ListOrdersRequest, opts ...grpc.CallOption) (*orderv1.ListOrdersResponse, error)
	UpdateStatus(ctx context.Context, in *orderv1.UpdateStatusRequest, opts ...grpc.CallOption) (*orderv1.Order, error)
	GetTodayStats(ctx context.Context, in *orderv1.GetTodayStatsRequest, opts ...grpc.CallOption) (*orderv1.TodayStats, error)
}

// LowStockService adalah klien gRPC katalog untuk kartu stok menipis.
type LowStockService interface {
	ListLowStock(ctx context.Context, in *catalogv1.ListLowStockRequest, opts ...grpc.CallOption) (*catalogv1.ListLowStockResponse, error)
}

// Admin menangani endpoint panel admin (pesanan & ringkasan).
type Admin struct {
	orders  AdminOrderService
	catalog LowStockService
	log     *slog.Logger
}

// NewAdmin membuat handler panel admin.
func NewAdmin(orders AdminOrderService, catalog LowStockService, log *slog.Logger) *Admin {
	return &Admin{orders: orders, catalog: catalog, log: log}
}

type updateStatusBody struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

type lowStockJSON struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Stock     int64  `json:"stock"`
	Unit      string `json:"unit"`
}

type todayStatsJSON struct {
	OrdersCount          int64          `json:"orders_count"`
	RevenuePaid          int64          `json:"revenue_paid"`
	AwaitingVerification int64          `json:"awaiting_verification"`
	LowStock             []lowStockJSON `json:"low_stock"`
}

// ListOrders menangani GET /api/v1/admin/orders.
func (h *Admin) ListOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.orders.ListOrders(outgoingContext(r), &orderv1.ListOrdersRequest{
		Status:   q.Get("status"),
		DateFrom: q.Get("date_from"),
		DateTo:   q.Get("date_to"),
		Q:        q.Get("q"),
		Page:     int32(queryInt(r, "page", 1)),
		Limit:    int32(queryInt(r, "limit", 20)),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	out := make([]orderJSON, 0, len(res.GetOrders()))
	for _, o := range res.GetOrders() {
		out = append(out, toOrderJSON(o))
	}
	httpx.DataMeta(w, http.StatusOK, out, map[string]any{
		"page":  res.GetPage(),
		"limit": res.GetLimit(),
		"total": res.GetTotal(),
	})
}

// GetOrder menangani GET /api/v1/admin/orders/{id}.
func (h *Admin) GetOrder(w http.ResponseWriter, r *http.Request) {
	res, err := h.orders.GetOrderByID(outgoingContext(r), &orderv1.GetOrderByIDRequest{
		Id: chiURLParam(r, "id"),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toOrderJSON(res))
}

// UpdateStatus menangani PATCH /api/v1/admin/orders/{id}/status.
func (h *Admin) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var body updateStatusBody
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.orders.UpdateStatus(outgoingContext(r), &orderv1.UpdateStatusRequest{
		Id:     chiURLParam(r, "id"),
		Status: body.Status,
		Note:   body.Note,
		Actor:  identityUserID(r),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toOrderJSON(res))
}

// TodayStats menangani GET /api/v1/admin/stats/today.
//
// Ringkasan pesanan berasal dari order service; daftar stok menipis digabung
// dari catalog service (satu DB per service, digabung di gateway).
func (h *Admin) TodayStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.orders.GetTodayStats(outgoingContext(r), &orderv1.GetTodayStatsRequest{})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	out := todayStatsJSON{
		OrdersCount:          stats.GetOrdersCount(),
		RevenuePaid:          stats.GetRevenuePaid(),
		AwaitingVerification: stats.GetAwaitingVerification(),
		LowStock:             []lowStockJSON{},
	}
	low, err := h.catalog.ListLowStock(outgoingContext(r), &catalogv1.ListLowStockRequest{Limit: 10})
	if err != nil {
		// Stok menipis bersifat pelengkap; kegagalan tidak menggagalkan ringkasan.
		h.log.Warn("ambil stok menipis gagal", "error", err)
	} else {
		for _, p := range low.GetProducts() {
			out.LowStock = append(out.LowStock, lowStockJSON{
				ProductID: p.GetProductId(),
				Name:      p.GetName(),
				Stock:     p.GetStock(),
				Unit:      p.GetUnit(),
			})
		}
	}
	httpx.Data(w, http.StatusOK, out)
}
