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
	"google.golang.org/protobuf/types/known/emptypb"

	shippingv1 "saybu/gen/saybu/shipping/v1"
)

type fakeShipping struct {
	listZones  func(context.Context, *shippingv1.ListZonesRequest) (*shippingv1.ListZonesResponse, error)
	quote      func(context.Context, *shippingv1.QuoteRequest) (*shippingv1.QuoteResponse, error)
	upsertZone func(context.Context, *shippingv1.UpsertZoneRequest) (*shippingv1.Zone, error)
	deleteZone func(context.Context, *shippingv1.DeleteZoneRequest) (*emptypb.Empty, error)
}

func (f *fakeShipping) ListZones(ctx context.Context, in *shippingv1.ListZonesRequest, _ ...grpc.CallOption) (*shippingv1.ListZonesResponse, error) {
	return f.listZones(ctx, in)
}
func (f *fakeShipping) Quote(ctx context.Context, in *shippingv1.QuoteRequest, _ ...grpc.CallOption) (*shippingv1.QuoteResponse, error) {
	return f.quote(ctx, in)
}
func (f *fakeShipping) UpsertZone(ctx context.Context, in *shippingv1.UpsertZoneRequest, _ ...grpc.CallOption) (*shippingv1.Zone, error) {
	return f.upsertZone(ctx, in)
}
func (f *fakeShipping) DeleteZone(ctx context.Context, in *shippingv1.DeleteZoneRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.deleteZone(ctx, in)
}

func newShippingHandler(client ShippingService) *Shipping {
	return NewShipping(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func sampleZone() *shippingv1.Zone {
	return &shippingv1.Zone{
		Id: "z-1", Name: "Kec. Sukamaju", Areas: []string{"Sukamaju", "Sukamulya"},
		Fee: 10000, FreeMinSubtotal: 150000, IsActive: true,
		CreatedAt: "2026-11-13T10:00:00Z", UpdatedAt: "2026-11-13T10:00:00Z",
	}
}

func TestListZonesPublicShape(t *testing.T) {
	var got *shippingv1.ListZonesRequest
	h := newShippingHandler(&fakeShipping{
		listZones: func(_ context.Context, in *shippingv1.ListZonesRequest) (*shippingv1.ListZonesResponse, error) {
			got = in
			return &shippingv1.ListZonesResponse{Zones: []*shippingv1.Zone{sampleZone()}}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.ListZones(rec, httptest.NewRequest(http.MethodGet, "/api/v1/shipping/zones", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetIncludeInactive() {
		t.Fatal("endpoint publik tidak boleh menyertakan zona nonaktif")
	}
	item, _ := decodeBody(t, rec)["data"].([]any)[0].(map[string]any)
	if item["fee"] != float64(10000) || item["free_min_subtotal"] != float64(150000) {
		t.Fatalf("item = %+v", item)
	}
	if _, ok := item["is_active"]; ok {
		t.Fatal("respons publik tidak boleh memuat is_active")
	}
}

func TestAdminListZonesIncludesInactive(t *testing.T) {
	var got *shippingv1.ListZonesRequest
	h := newShippingHandler(&fakeShipping{
		listZones: func(_ context.Context, in *shippingv1.ListZonesRequest) (*shippingv1.ListZonesResponse, error) {
			got = in
			return &shippingv1.ListZonesResponse{Zones: []*shippingv1.Zone{sampleZone()}}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.AdminListZones(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/shipping/zones", nil))

	if !got.GetIncludeInactive() {
		t.Fatal("endpoint admin harus menyertakan zona nonaktif")
	}
	item, _ := decodeBody(t, rec)["data"].([]any)[0].(map[string]any)
	if item["is_active"] != true {
		t.Fatalf("item = %+v", item)
	}
}

func TestQuote(t *testing.T) {
	var got *shippingv1.QuoteRequest
	h := newShippingHandler(&fakeShipping{
		quote: func(_ context.Context, in *shippingv1.QuoteRequest) (*shippingv1.QuoteResponse, error) {
			got = in
			return &shippingv1.QuoteResponse{
				ZoneId: "z-1", ZoneName: "Kec. Sukamaju", Fee: 10000,
				Subtotal: 75000, Total: 85000,
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.Quote(rec, httptest.NewRequest(http.MethodGet, "/api/v1/shipping/quote?zone_id=z-1&subtotal=75000", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetZoneId() != "z-1" || got.GetSubtotal() != 75000 {
		t.Fatalf("request = %+v", got)
	}
	data, _ := decodeBody(t, rec)["data"].(map[string]any)
	if data["total"] != float64(85000) || data["free_shipping"] != false {
		t.Fatalf("data = %+v", data)
	}
}

func TestQuoteUnknownZoneMaps422(t *testing.T) {
	h := newShippingHandler(&fakeShipping{
		quote: func(context.Context, *shippingv1.QuoteRequest) (*shippingv1.QuoteResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "Zona tidak dikenal.")
		},
	})

	rec := httptest.NewRecorder()
	h.Quote(rec, httptest.NewRequest(http.MethodGet, "/api/v1/shipping/quote?zone_id=x&subtotal=1", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestCreateZone(t *testing.T) {
	var got *shippingv1.UpsertZoneRequest
	h := newShippingHandler(&fakeShipping{
		upsertZone: func(_ context.Context, in *shippingv1.UpsertZoneRequest) (*shippingv1.Zone, error) {
			got = in
			return sampleZone(), nil
		},
	})

	payload := `{"name":"Kec. Sukamaju","areas":["Sukamaju"],"fee":10000,"free_min_subtotal":150000}`
	rec := httptest.NewRecorder()
	h.CreateZone(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/shipping/zones", strings.NewReader(payload)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if got.GetId() != "" || got.GetFee() != 10000 || len(got.GetAreas()) != 1 {
		t.Fatalf("request = %+v", got)
	}
}

func TestUpdateZone(t *testing.T) {
	var got *shippingv1.UpsertZoneRequest
	h := newShippingHandler(&fakeShipping{
		upsertZone: func(_ context.Context, in *shippingv1.UpsertZoneRequest) (*shippingv1.Zone, error) {
			got = in
			return sampleZone(), nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodPatch, "/api/v1/admin/shipping/zones/z-1",
		strings.NewReader(`{"name":"Kec. Sukamaju","fee":12000}`)), "id", "z-1")
	h.UpdateZone(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got.GetId() != "z-1" || got.GetFee() != 12000 {
		t.Fatalf("request = %+v", got)
	}
}

func TestDeleteZone(t *testing.T) {
	var gotID string
	h := newShippingHandler(&fakeShipping{
		deleteZone: func(_ context.Context, in *shippingv1.DeleteZoneRequest) (*emptypb.Empty, error) {
			gotID = in.GetId()
			return &emptypb.Empty{}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := withParam(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/shipping/zones/z-1", nil), "id", "z-1")
	h.DeleteZone(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if gotID != "z-1" {
		t.Fatalf("id = %q", gotID)
	}
}

func TestShippingRejectsUnknownFields(t *testing.T) {
	h := newShippingHandler(&fakeShipping{})

	rec := httptest.NewRecorder()
	h.CreateZone(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/shipping/zones",
		strings.NewReader(`{"name":"X","tidak_dikenal":1}`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
