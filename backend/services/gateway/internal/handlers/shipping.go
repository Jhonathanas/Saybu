package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	shippingv1 "saybu/gen/saybu/shipping/v1"
	"saybu/pkg/httpx"
)

// ShippingService adalah bagian klien gRPC shipping yang dipakai gateway.
type ShippingService interface {
	ListZones(ctx context.Context, in *shippingv1.ListZonesRequest, opts ...grpc.CallOption) (*shippingv1.ListZonesResponse, error)
	Quote(ctx context.Context, in *shippingv1.QuoteRequest, opts ...grpc.CallOption) (*shippingv1.QuoteResponse, error)
	UpsertZone(ctx context.Context, in *shippingv1.UpsertZoneRequest, opts ...grpc.CallOption) (*shippingv1.Zone, error)
	DeleteZone(ctx context.Context, in *shippingv1.DeleteZoneRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

// Shipping menangani endpoint zona ongkir publik & admin.
type Shipping struct {
	client ShippingService
	log    *slog.Logger
}

// NewShipping membuat handler ongkir.
func NewShipping(client ShippingService, log *slog.Logger) *Shipping {
	return &Shipping{client: client, log: log}
}

type zoneJSON struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Areas           []string `json:"areas"`
	Fee             int64    `json:"fee"`
	FreeMinSubtotal int64    `json:"free_min_subtotal"`
	IsActive        *bool    `json:"is_active,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
}

type quoteJSON struct {
	ZoneID       string `json:"zone_id"`
	ZoneName     string `json:"zone_name"`
	Fee          int64  `json:"fee"`
	FreeShipping bool   `json:"free_shipping"`
	Subtotal     int64  `json:"subtotal"`
	Total        int64  `json:"total"`
}

type zoneBody struct {
	Name            string   `json:"name"`
	Areas           []string `json:"areas"`
	Fee             int64    `json:"fee"`
	FreeMinSubtotal int64    `json:"free_min_subtotal"`
	IsActive        *bool    `json:"is_active"`
}

// ListZones menangani GET /api/v1/shipping/zones (hanya zona aktif).
func (h *Shipping) ListZones(w http.ResponseWriter, r *http.Request) {
	h.listZones(w, r, false)
}

// AdminListZones menangani GET /api/v1/admin/shipping/zones (termasuk nonaktif).
func (h *Shipping) AdminListZones(w http.ResponseWriter, r *http.Request) {
	h.listZones(w, r, true)
}

func (h *Shipping) listZones(w http.ResponseWriter, r *http.Request, admin bool) {
	res, err := h.client.ListZones(outgoingContext(r), &shippingv1.ListZonesRequest{IncludeInactive: admin})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	out := make([]zoneJSON, 0, len(res.GetZones()))
	for _, z := range res.GetZones() {
		out = append(out, *toZoneJSON(z, admin))
	}
	httpx.Data(w, http.StatusOK, out)
}

// Quote menangani GET /api/v1/shipping/quote?zone_id=...&subtotal=...
func (h *Shipping) Quote(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.client.Quote(outgoingContext(r), &shippingv1.QuoteRequest{
		ZoneId:   q.Get("zone_id"),
		Subtotal: queryInt64(r, "subtotal", 0),
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, quoteJSON{
		ZoneID:       res.GetZoneId(),
		ZoneName:     res.GetZoneName(),
		Fee:          res.GetFee(),
		FreeShipping: res.GetFreeShipping(),
		Subtotal:     res.GetSubtotal(),
		Total:        res.GetTotal(),
	})
}

// CreateZone menangani POST /api/v1/admin/shipping/zones.
func (h *Shipping) CreateZone(w http.ResponseWriter, r *http.Request) {
	var body zoneBody
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.client.UpsertZone(outgoingContext(r), &shippingv1.UpsertZoneRequest{
		Name:            body.Name,
		Areas:           body.Areas,
		Fee:             body.Fee,
		FreeMinSubtotal: body.FreeMinSubtotal,
		IsActive:        body.IsActive,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusCreated, toZoneJSON(res, true))
}

// UpdateZone menangani PATCH /api/v1/admin/shipping/zones/{id}.
func (h *Shipping) UpdateZone(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var body zoneBody
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.client.UpsertZone(outgoingContext(r), &shippingv1.UpsertZoneRequest{
		Id:              id,
		Name:            body.Name,
		Areas:           body.Areas,
		Fee:             body.Fee,
		FreeMinSubtotal: body.FreeMinSubtotal,
		IsActive:        body.IsActive,
	})
	if err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	httpx.Data(w, http.StatusOK, toZoneJSON(res, true))
}

// DeleteZone menangani DELETE /api/v1/admin/shipping/zones/{id}.
func (h *Shipping) DeleteZone(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	if _, err := h.client.DeleteZone(outgoingContext(r), &shippingv1.DeleteZoneRequest{Id: id}); err != nil {
		WriteGRPCError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toZoneJSON(z *shippingv1.Zone, admin bool) *zoneJSON {
	out := &zoneJSON{
		ID:              z.GetId(),
		Name:            z.GetName(),
		Areas:           z.GetAreas(),
		Fee:             z.GetFee(),
		FreeMinSubtotal: z.GetFreeMinSubtotal(),
	}
	if out.Areas == nil {
		out.Areas = []string{}
	}
	if admin {
		active := z.GetIsActive()
		out.IsActive = &active
		out.CreatedAt = z.GetCreatedAt()
		out.UpdatedAt = z.GetUpdatedAt()
	}
	return out
}

// queryInt64 membaca query integer 64-bit; nilai tidak valid memakai default.
func queryInt64(r *http.Request, key string, def int64) int64 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return n
}
