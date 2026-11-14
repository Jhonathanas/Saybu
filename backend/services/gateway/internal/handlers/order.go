package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/grpc"

	orderv1 "saybu/gen/saybu/order/v1"
	"saybu/pkg/httpx"
	"saybu/services/gateway/internal/authmw"
	"saybu/services/gateway/internal/ratelimit"
)

// OrderService adalah bagian klien gRPC order yang dipakai gateway.
type OrderService interface {
	CreateOrder(ctx context.Context, in *orderv1.CreateOrderRequest, opts ...grpc.CallOption) (*orderv1.Order, error)
	GetOrderByCode(ctx context.Context, in *orderv1.GetOrderByCodeRequest, opts ...grpc.CallOption) (*orderv1.Order, error)
	ListMyOrders(ctx context.Context, in *orderv1.ListMyOrdersRequest, opts ...grpc.CallOption) (*orderv1.ListOrdersResponse, error)
	CancelOrder(ctx context.Context, in *orderv1.CancelOrderRequest, opts ...grpc.CallOption) (*orderv1.Order, error)
}

// Order menangani endpoint checkout, pelacakan, dan pembatalan.
type Order struct {
	client      OrderService
	log         *slog.Logger
	createLimit *ratelimit.Limiter
}

// NewOrder membuat handler pesanan. createLimit membatasi POST /orders per IP.
func NewOrder(client OrderService, log *slog.Logger, createLimit *ratelimit.Limiter) *Order {
	return &Order{client: client, log: log, createLimit: createLimit}
}

type customerJSON struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email,omitempty"`
	Note  string `json:"note,omitempty"`
}

type addressJSON struct {
	Line       string `json:"line,omitempty"`
	RTRW       string `json:"rt_rw,omitempty"`
	Kelurahan  string `json:"kelurahan,omitempty"`
	Kecamatan  string `json:"kecamatan,omitempty"`
	City       string `json:"city,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Landmark   string `json:"landmark,omitempty"`
}

type orderItemJSON struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	Qty        int64  `json:"qty"`
	QtyDisplay string `json:"qty_display"`
	UnitPrice  int64  `json:"unit_price"`
	LineTotal  int64  `json:"line_total"`
}

type statusHistoryJSON struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
	At     string `json:"at"`
}

type orderJSON struct {
	ID            string              `json:"id"`
	Code          string              `json:"code"`
	Status        string              `json:"status"`
	Fulfillment   string              `json:"fulfillment"`
	PaymentMethod string              `json:"payment_method"`
	Customer      customerJSON        `json:"customer"`
	Address       addressJSON         `json:"address"`
	ZoneID        string              `json:"zone_id,omitempty"`
	ZoneName      string              `json:"zone_name,omitempty"`
	Subtotal      int64               `json:"subtotal"`
	ShippingFee   int64               `json:"shipping_fee"`
	Total         int64               `json:"total"`
	Items         []orderItemJSON     `json:"items"`
	StatusHistory []statusHistoryJSON `json:"status_history"`
	CancelReason  string              `json:"cancel_reason,omitempty"`
	ExpiresAt     string              `json:"expires_at"`
	CreatedAt     string              `json:"created_at"`
}

type createOrderBody struct {
	Customer struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
		Email string `json:"email"`
		Note  string `json:"note"`
	} `json:"customer"`
	Fulfillment   string      `json:"fulfillment"`
	ZoneID        string      `json:"zone_id"`
	Address       addressJSON `json:"address"`
	PaymentMethod string      `json:"payment_method"`
	Items         []struct {
		ProductID string `json:"product_id"`
		Qty       int64  `json:"qty"`
	} `json:"items"`
}

type cancelOrderBody struct {
	Phone  string `json:"phone"`
	Reason string `json:"reason"`
}

// CreateOrder menangani POST /api/v1/orders (guest & akun).
func (h *Order) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if !allowRate(w, r, h.createLimit, "order:"+clientIP(r)) {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		httpx.WriteError(w, r, httpx.BadRequest("Header Idempotency-Key wajib diisi.",
			httpx.Detail{Field: "Idempotency-Key", Message: "Wajib diisi untuk mencegah pesanan ganda."}))
		return
	}

	var body createOrderBody
	if !decodeJSON(w, r, &body) {
		return
	}

	req := &orderv1.CreateOrderRequest{
		Customer: &orderv1.Customer{
			Name:  body.Customer.Name,
			Phone: body.Customer.Phone,
			Email: body.Customer.Email,
			Note:  body.Customer.Note,
		},
		Fulfillment: body.Fulfillment,
		ZoneId:      body.ZoneID,
		Address: &orderv1.Address{
			Line:       body.Address.Line,
			RtRw:       body.Address.RTRW,
			Kelurahan:  body.Address.Kelurahan,
			Kecamatan:  body.Address.Kecamatan,
			City:       body.Address.City,
			PostalCode: body.Address.PostalCode,
			Landmark:   body.Address.Landmark,
		},
		PaymentMethod:  body.PaymentMethod,
		UserId:         identityUserID(r),
		IdempotencyKey: key,
	}
	for _, it := range body.Items {
		req.Items = append(req.Items, &orderv1.OrderItemInput{ProductId: it.ProductID, Qty: it.Qty})
	}

	res, err := h.client.CreateOrder(outgoingContext(r), req)
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusCreated, toOrderJSON(res))
}

// GetOrder menangani GET /api/v1/orders/{code}?phone=...
func (h *Order) GetOrder(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetOrderByCode(outgoingContext(r), &orderv1.GetOrderByCodeRequest{
		Code:   chiURLParam(r, "code"),
		Phone:  r.URL.Query().Get("phone"),
		UserId: identityUserID(r),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toOrderJSON(res))
}

// ListMyOrders menangani GET /api/v1/me/orders (login wajib).
func (h *Order) ListMyOrders(w http.ResponseWriter, r *http.Request) {
	id, ok := authmw.IdentityFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.Unauthorized("Token akses diperlukan."))
		return
	}
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	res, err := h.client.ListMyOrders(outgoingContext(r), &orderv1.ListMyOrdersRequest{
		UserId: id.UserID,
		Page:   int32(page),
		Limit:  int32(limit),
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

// CancelOrder menangani POST /api/v1/orders/{code}/cancel.
func (h *Order) CancelOrder(w http.ResponseWriter, r *http.Request) {
	var body cancelOrderBody
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.client.CancelOrder(outgoingContext(r), &orderv1.CancelOrderRequest{
		Code:   chiURLParam(r, "code"),
		Phone:  body.Phone,
		UserId: identityUserID(r),
		Reason: body.Reason,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toOrderJSON(res))
}

func identityUserID(r *http.Request) string {
	if id, ok := authmw.IdentityFrom(r.Context()); ok {
		return id.UserID
	}
	return ""
}

func toOrderJSON(o *orderv1.Order) orderJSON {
	out := orderJSON{
		ID:            o.GetId(),
		Code:          o.GetCode(),
		Status:        o.GetStatus(),
		Fulfillment:   o.GetFulfillment(),
		PaymentMethod: o.GetPaymentMethod(),
		Customer: customerJSON{
			Name:  o.GetCustomer().GetName(),
			Phone: o.GetCustomer().GetPhone(),
			Email: o.GetCustomer().GetEmail(),
			Note:  o.GetCustomer().GetNote(),
		},
		Address: addressJSON{
			Line:       o.GetAddress().GetLine(),
			RTRW:       o.GetAddress().GetRtRw(),
			Kelurahan:  o.GetAddress().GetKelurahan(),
			Kecamatan:  o.GetAddress().GetKecamatan(),
			City:       o.GetAddress().GetCity(),
			PostalCode: o.GetAddress().GetPostalCode(),
			Landmark:   o.GetAddress().GetLandmark(),
		},
		ZoneID:       o.GetZoneId(),
		ZoneName:     o.GetZoneName(),
		Subtotal:     o.GetSubtotal(),
		ShippingFee:  o.GetShippingFee(),
		Total:        o.GetTotal(),
		CancelReason: o.GetCancelReason(),
		ExpiresAt:    o.GetExpiresAt(),
		CreatedAt:    o.GetCreatedAt(),
	}
	out.Items = make([]orderItemJSON, 0, len(o.GetItems()))
	for _, it := range o.GetItems() {
		out.Items = append(out.Items, orderItemJSON{
			ProductID:  it.GetProductId(),
			Name:       it.GetName(),
			Unit:       it.GetUnit(),
			Qty:        it.GetQty(),
			QtyDisplay: it.GetQtyDisplay(),
			UnitPrice:  it.GetUnitPrice(),
			LineTotal:  it.GetLineTotal(),
		})
	}
	out.StatusHistory = make([]statusHistoryJSON, 0, len(o.GetStatusHistory()))
	for _, hs := range o.GetStatusHistory() {
		out.StatusHistory = append(out.StatusHistory, statusHistoryJSON{
			Status: hs.GetStatus(),
			Note:   hs.GetNote(),
			At:     hs.GetAt(),
		})
	}
	return out
}
