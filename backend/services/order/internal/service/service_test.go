package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	shippingv1 "saybu/gen/saybu/shipping/v1"
	"saybu/pkg/kafkax"
	"saybu/pkg/outbox"
	"saybu/services/order/internal/domain"
	"saybu/services/order/internal/repo"
)

type fakeRepo struct {
	orders    map[string]domain.Order
	byCode    map[string]string
	idem      map[string]repo.Idempotency
	seq       int64
	createErr error

	createEvents     []outbox.Record
	transitionEvents []outbox.Record
	expireCount      int
	lastFrom, lastTo string
	idemCalls        int
	missFirstLookup  bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		orders: map[string]domain.Order{},
		byCode: map[string]string{},
		idem:   map[string]repo.Idempotency{},
	}
}

func (f *fakeRepo) GetIdempotency(_ context.Context, key string) (repo.Idempotency, bool, error) {
	f.idemCalls++
	if f.missFirstLookup && f.idemCalls == 1 {
		return repo.Idempotency{}, false, nil
	}
	it, ok := f.idem[key]
	return it, ok, nil
}

func (f *fakeRepo) GetOrderByCode(_ context.Context, code string) (domain.Order, error) {
	id, ok := f.byCode[code]
	if !ok {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	return f.orders[id], nil
}

func (f *fakeRepo) GetOrderByID(_ context.Context, id string) (domain.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	return o, nil
}

func (f *fakeRepo) ListOrdersByUser(_ context.Context, userID string, _, _ int) ([]domain.Order, int64, error) {
	var out []domain.Order
	for _, o := range f.orders {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) CreateOrder(_ context.Context, o domain.Order, idem repo.Idempotency, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, error) {
	if f.createErr != nil {
		return domain.Order{}, f.createErr
	}
	f.seq++
	o.Code = fmt.Sprintf("SB-20261114-%04d", f.seq)
	o.Status = domain.StatusPendingPayment
	o.CreatedAt = time.Now().UTC()
	o.UpdatedAt = o.CreatedAt
	if events != nil {
		recs, err := events(o)
		if err != nil {
			return domain.Order{}, err
		}
		f.createEvents = append(f.createEvents, recs...)
	}
	f.orders[o.ID] = o
	f.byCode[o.Code] = o.ID
	if idem.Key != "" {
		idem.OrderID = o.ID
		f.idem[idem.Key] = idem
	}
	return o, nil
}

func (f *fakeRepo) Transition(_ context.Context, id, from, to, note, actor string, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if o.Status != from {
		return domain.Order{}, domain.ErrInvalidTransition
	}
	f.lastFrom, f.lastTo = from, to
	o.Status = to
	if to == domain.StatusCancelled {
		o.CancelReason = note
	}
	o.History = append(o.History, domain.StatusEntry{Status: to, Note: note, Actor: actor, At: time.Now().UTC()})
	if events != nil {
		recs, err := events(o)
		if err != nil {
			return domain.Order{}, err
		}
		f.transitionEvents = append(f.transitionEvents, recs...)
	}
	f.orders[id] = o
	return o, nil
}

func (f *fakeRepo) ExpireDue(_ context.Context, _ int, events func(domain.Order) ([]outbox.Record, error)) ([]domain.Order, error) {
	return nil, nil
}

func (f *fakeRepo) TransitionFromPayment(_ context.Context, eventID, eventType, orderID, from, to, note, actor string, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, bool, error) {
	o, ok := f.orders[orderID]
	if !ok {
		return domain.Order{}, false, domain.ErrOrderNotFound
	}
	if o.Status != from {
		return domain.Order{}, false, nil
	}
	f.lastFrom, f.lastTo = from, to
	o.Status = to
	o.History = append(o.History, domain.StatusEntry{Status: to, Note: note, Actor: actor, At: time.Now().UTC()})
	if events != nil {
		recs, err := events(o)
		if err != nil {
			return domain.Order{}, false, err
		}
		f.transitionEvents = append(f.transitionEvents, recs...)
	}
	f.orders[orderID] = o
	return o, true, nil
}

type fakeCatalog struct {
	products     map[string]*catalogv1.Product
	stock        map[string]int64
	reserved     map[string][]*catalogv1.StockItem
	releaseCalls []string
	getErr       error
	releaseErr   error
}

func newFakeCatalog() *fakeCatalog {
	return &fakeCatalog{
		products: map[string]*catalogv1.Product{},
		stock:    map[string]int64{},
		reserved: map[string][]*catalogv1.StockItem{},
	}
}

func (f *fakeCatalog) GetProduct(_ context.Context, in *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.Product, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.products[in.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "produk tidak ditemukan")
	}
	return p, nil
}

func (f *fakeCatalog) CheckStock(_ context.Context, in *catalogv1.CheckStockRequest, _ ...grpc.CallOption) (*catalogv1.CheckStockResponse, error) {
	res := &catalogv1.CheckStockResponse{AllOk: true}
	for _, it := range in.GetItems() {
		avail := f.stock[it.GetProductId()]
		st := &catalogv1.StockStatus{ProductId: it.GetProductId(), Requested: it.GetQty(), Available: avail, Ok: avail >= it.GetQty()}
		if !st.Ok {
			res.AllOk = false
		}
		res.Items = append(res.Items, st)
	}
	return res, nil
}

func (f *fakeCatalog) ReserveStock(_ context.Context, in *catalogv1.ReserveStockRequest, _ ...grpc.CallOption) (*catalogv1.ReserveStockResponse, error) {
	res := &catalogv1.ReserveStockResponse{Reserved: true}
	for _, it := range in.GetItems() {
		avail := f.stock[it.GetProductId()]
		if avail < it.GetQty() {
			res.Reserved = false
			res.Insufficient = append(res.Insufficient, &catalogv1.StockStatus{
				ProductId: it.GetProductId(), Requested: it.GetQty(), Available: avail,
			})
			continue
		}
		f.stock[it.GetProductId()] = avail - it.GetQty()
		f.reserved[in.GetOrderId()] = append(f.reserved[in.GetOrderId()], it)
	}
	return res, nil
}

func (f *fakeCatalog) ReleaseStock(_ context.Context, in *catalogv1.ReleaseStockRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.releaseCalls = append(f.releaseCalls, in.GetOrderId())
	if f.releaseErr != nil {
		return nil, f.releaseErr
	}
	for _, it := range f.reserved[in.GetOrderId()] {
		f.stock[it.GetProductId()] += it.GetQty()
	}
	delete(f.reserved, in.GetOrderId())
	return &emptypb.Empty{}, nil
}

type fakeShipping struct {
	fee      int64
	zoneName string
	err      error
	calls    int
}

func (f *fakeShipping) Quote(_ context.Context, in *shippingv1.QuoteRequest, _ ...grpc.CallOption) (*shippingv1.QuoteResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &shippingv1.QuoteResponse{
		ZoneId:   in.GetZoneId(),
		ZoneName: f.zoneName,
		Fee:      f.fee,
		Subtotal: in.GetSubtotal(),
		Total:    in.GetSubtotal() + f.fee,
	}, nil
}

func newTestService(t *testing.T) (*Service, *fakeRepo, *fakeCatalog, *fakeShipping) {
	t.Helper()
	r := newFakeRepo()
	c := newFakeCatalog()
	s := &fakeShipping{fee: 10000, zoneName: "Kec. Sukamaju"}
	svc, err := New(r, c, s, nil, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Date(2026, 11, 14, 3, 0, 0, 0, time.UTC) }
	return svc, r, c, s
}

func addProduct(c *fakeCatalog, id, name, unit string, price, stock, min, step int64) {
	c.products[id] = &catalogv1.Product{
		Id: id, Name: name, Unit: unit, Price: price,
		Stock: stock, MinQty: min, StepQty: step, IsActive: true,
	}
	c.stock[id] = stock
}

func baseInput() domain.CreateInput {
	return domain.CreateInput{
		Customer:      domain.Customer{Name: "Siti", Phone: "081234567890"},
		Fulfillment:   domain.FulfillmentDelivery,
		ZoneID:        "zone-1",
		Address:       domain.Address{Line: "Jl. Melati 12"},
		PaymentMethod: domain.PaymentTransfer,
		Items:         []domain.ItemInput{{ProductID: "p1", Qty: 2}},
	}
}

func TestCreateOrderHappyPath(t *testing.T) {
	svc, r, c, sh := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)
	addProduct(c, "p2", "Tomat", "kg", 15000, 5000, 100, 100)

	in := baseInput()
	in.Items = []domain.ItemInput{{ProductID: "p1", Qty: 2}, {ProductID: "p2", Qty: 1000}}

	o, err := svc.CreateOrder(context.Background(), in, "key-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Code != "SB-20261114-0001" {
		t.Fatalf("code = %q", o.Code)
	}
	// subtotal = 2*5000 + 15000 = 25000 ; ongkir 10000 ; total 35000
	if o.Subtotal != 25000 || o.ShippingFee != 10000 || o.Total != 35000 {
		t.Fatalf("nominal salah: subtotal=%d fee=%d total=%d", o.Subtotal, o.ShippingFee, o.Total)
	}
	if len(o.Items) != 2 || domain.DisplayQty(o.Items[1].Unit, o.Items[1].Qty) != "1 kg" {
		t.Fatalf("items salah: %+v", o.Items)
	}
	if sh.calls != 1 {
		t.Fatalf("quote dipanggil %d kali", sh.calls)
	}
	if c.stock["p1"] != 8 || c.stock["p2"] != 4000 {
		t.Fatalf("stok tidak ter-reserve: %+v", c.stock)
	}
	if len(r.createEvents) != 1 || r.createEvents[0].Headers["event-type"] != "order.created" {
		t.Fatalf("event order.created tidak ditulis: %+v", r.createEvents)
	}
	if _, ok := r.idem["key-1"]; !ok {
		t.Fatal("kunci idempotensi tidak disimpan")
	}
	if o.ExpiresAt.Sub(time.Date(2026, 11, 14, 3, 0, 0, 0, time.UTC)) != 24*time.Hour {
		t.Fatalf("expires_at salah: %v", o.ExpiresAt)
	}
}

func TestCreateOrderPickupNoShipping(t *testing.T) {
	svc, _, c, sh := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	in := baseInput()
	in.Fulfillment = domain.FulfillmentPickup
	in.ZoneID = ""
	in.Address = domain.Address{}

	o, err := svc.CreateOrder(context.Background(), in, "key-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.ShippingFee != 0 || o.Total != 10000 {
		t.Fatalf("pickup harus gratis: fee=%d total=%d", o.ShippingFee, o.Total)
	}
	if sh.calls != 0 {
		t.Fatalf("quote tidak boleh dipanggil untuk pickup")
	}
}

func TestCreateOrderRequiresIdempotencyKey(t *testing.T) {
	svc, _, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	_, err := svc.CreateOrder(context.Background(), baseInput(), "")
	if !errors.Is(err, domain.ErrIdempotencyKeyMissing) {
		t.Fatalf("err = %v, want ErrIdempotencyKeyMissing", err)
	}
}

func TestCreateOrderRejectsBadQty(t *testing.T) {
	svc, _, c, _ := newTestService(t)
	addProduct(c, "p1", "Tomat", "kg", 15000, 5000, 100, 100)

	in := baseInput()
	in.Items = []domain.ItemInput{{ProductID: "p1", Qty: 150}} // bukan kelipatan 100

	_, err := svc.CreateOrder(context.Background(), in, "key-3")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestCreateOrderInsufficientStock(t *testing.T) {
	svc, r, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 1, 1, 1)

	in := baseInput()
	in.Items = []domain.ItemInput{{ProductID: "p1", Qty: 2}}

	_, err := svc.CreateOrder(context.Background(), in, "key-4")
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("err = %v, want ErrInsufficientStock", err)
	}
	var ise *domain.InsufficientStockError
	if !errors.As(err, &ise) || len(ise.Items) != 1 {
		t.Fatalf("detail stok kurang tidak ada: %+v", err)
	}
	if len(r.orders) != 0 {
		t.Fatal("pesanan tidak boleh tersimpan saat stok kurang")
	}
	if len(c.releaseCalls) != 0 {
		t.Fatal("tidak ada reservasi yang perlu dilepas")
	}
}

func TestCreateOrderCompensatesOnSaveFailure(t *testing.T) {
	svc, r, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)
	r.createErr = errors.New("db mati")

	_, err := svc.CreateOrder(context.Background(), baseInput(), "key-5")
	if err == nil {
		t.Fatal("harus gagal")
	}
	if len(c.releaseCalls) != 1 {
		t.Fatalf("kompensasi release tidak dijalankan: %v", c.releaseCalls)
	}
	if c.stock["p1"] != 10 {
		t.Fatalf("stok harus kembali: %d", c.stock["p1"])
	}
}

func TestCreateOrderIdempotentReplay(t *testing.T) {
	svc, r, c, sh := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	first, err := svc.CreateOrder(context.Background(), baseInput(), "same-key")
	if err != nil {
		t.Fatal(err)
	}
	sh.calls = 0

	second, err := svc.CreateOrder(context.Background(), baseInput(), "same-key")
	if err != nil {
		t.Fatalf("replay harus sukses: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("replay harus mengembalikan pesanan sama: %s vs %s", second.ID, first.ID)
	}
	if sh.calls != 0 {
		t.Fatal("replay tidak boleh memanggil downstream")
	}
	if len(r.orders) != 1 {
		t.Fatalf("hanya boleh ada satu pesanan: %d", len(r.orders))
	}
}

func TestCreateOrderIdempotencyMismatch(t *testing.T) {
	svc, _, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)
	addProduct(c, "p2", "Tomat", "kg", 15000, 5000, 100, 100)

	if _, err := svc.CreateOrder(context.Background(), baseInput(), "same-key"); err != nil {
		t.Fatal(err)
	}

	in := baseInput()
	in.Items = []domain.ItemInput{{ProductID: "p2", Qty: 1000}} // isi berbeda
	_, err := svc.CreateOrder(context.Background(), in, "same-key")
	if !errors.Is(err, domain.ErrIdempotencyMismatch) {
		t.Fatalf("err = %v, want ErrIdempotencyMismatch", err)
	}
}

func TestCreateOrderIdempotencyRaceReleasesStock(t *testing.T) {
	svc, r, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	// Simulasikan request lain menang: lookup pertama kosong, CreateOrder konflik,
	// lalu lookup berikutnya menemukan kunci milik request pemenang.
	r.missFirstLookup = true
	r.createErr = domain.ErrIdempotencyExists
	r.idem["race-key"] = repo.Idempotency{Key: "race-key", RequestHash: requestHash(mustValidate(t, baseInput())), OrderID: "winner"}
	r.orders["winner"] = domain.Order{ID: "winner", Code: "SB-20261114-0009", Status: domain.StatusPendingPayment}

	got, err := svc.CreateOrder(context.Background(), baseInput(), "race-key")
	if err != nil {
		t.Fatalf("harus mengembalikan pesanan pemenang: %v", err)
	}
	if got.ID != "winner" {
		t.Fatalf("harus mengembalikan pesanan pemenang, dapat %s", got.ID)
	}
	if len(c.releaseCalls) != 1 {
		t.Fatalf("stok reservasi ganda harus dilepas: %v", c.releaseCalls)
	}
}

func mustValidate(t *testing.T, in domain.CreateInput) domain.CreateInput {
	t.Helper()
	out, err := domain.ValidateCreate(in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCancelOrder(t *testing.T) {
	svc, r, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	o, err := svc.CreateOrder(context.Background(), baseInput(), "cancel-key")
	if err != nil {
		t.Fatal(err)
	}

	cancelled, err := svc.CancelOrder(context.Background(), o.Code, "081234567890", "", "berubah pikiran")
	if err != nil {
		t.Fatalf("cancel gagal: %v", err)
	}
	if cancelled.Status != domain.StatusCancelled || cancelled.CancelReason != "berubah pikiran" {
		t.Fatalf("status = %s reason=%q", cancelled.Status, cancelled.CancelReason)
	}
	if len(r.transitionEvents) != 1 || r.transitionEvents[0].Headers["event-type"] != "order.cancelled" {
		t.Fatalf("event order.cancelled tidak ditulis: %+v", r.transitionEvents)
	}
}

func TestCancelOrderWrongPhone(t *testing.T) {
	svc, _, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	o, _ := svc.CreateOrder(context.Background(), baseInput(), "cancel-key-2")
	_, err := svc.CancelOrder(context.Background(), o.Code, "089999999999", "", "")
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("err = %v, want ErrOrderNotFound", err)
	}
}

func TestCancelOrderNotAllowed(t *testing.T) {
	svc, r, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	o, _ := svc.CreateOrder(context.Background(), baseInput(), "cancel-key-3")
	o.Status = domain.StatusShipped
	r.orders[o.ID] = o

	_, err := svc.CancelOrder(context.Background(), o.Code, "081234567890", "", "")
	if !errors.Is(err, domain.ErrCancelNotAllowed) {
		t.Fatalf("err = %v, want ErrCancelNotAllowed", err)
	}
}

func TestGetOrderByCodeOwnerNoPhone(t *testing.T) {
	svc, r, c, _ := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)

	in := baseInput()
	in.UserID = "user-9"
	o, _ := svc.CreateOrder(context.Background(), in, "owner-key")

	got, err := svc.GetOrderByCode(context.Background(), o.Code, "", "user-9")
	if err != nil {
		t.Fatalf("pemilik harus bisa lacak tanpa HP: %v", err)
	}
	if got.ID != o.ID {
		t.Fatal("pesanan salah")
	}
	_ = r
}

func TestCreateOrderUnknownZoneMapsBusinessRule(t *testing.T) {
	svc, _, c, sh := newTestService(t)
	addProduct(c, "p1", "Bayam", "ikat", 5000, 10, 1, 1)
	sh.err = status.Error(codes.FailedPrecondition, "Zona tidak dikenal.")

	_, err := svc.CreateOrder(context.Background(), baseInput(), "zone-key")
	if !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("err = %v, want ErrZoneNotFound", err)
	}
	// Zona divalidasi sebelum reservasi stok.
	if len(c.reserved) != 0 {
		t.Fatalf("stok tidak boleh direservasi: %+v", c.reserved)
	}
}

func paymentEvent(eventType string, payload map[string]any) kafkax.Event {
	raw, _ := json.Marshal(payload)
	return kafkax.Event{EventID: "ev-" + eventType, Type: eventType, Payload: raw}
}

func TestHandlePaymentEventTransitions(t *testing.T) {
	cases := []struct {
		eventType string
		from      string
		want      string
	}{
		{"payment.proof_uploaded", domain.StatusPendingPayment, domain.StatusWaitingVerification},
		{"payment.confirmed", domain.StatusWaitingVerification, domain.StatusPaid},
		{"payment.rejected", domain.StatusWaitingVerification, domain.StatusPendingPayment},
	}
	for _, c := range cases {
		t.Run(c.eventType, func(t *testing.T) {
			svc, r, _, _ := newTestService(t)
			r.orders["o-1"] = domain.Order{ID: "o-1", Code: "SB-1", Status: c.from}

			ev := paymentEvent(c.eventType, map[string]any{"order_id": "o-1", "code": "SB-1", "reason": "x"})
			if err := svc.HandlePaymentEvent(context.Background(), ev); err != nil {
				t.Fatal(err)
			}
			if got := r.orders["o-1"].Status; got != c.want {
				t.Fatalf("status = %s, want %s", got, c.want)
			}
			if len(r.transitionEvents) != 1 || r.transitionEvents[0].Headers["event-type"] != "order.status_changed" {
				t.Fatalf("event = %+v", r.transitionEvents)
			}
		})
	}
}

func TestHandlePaymentEventNoOpWhenWrongStatus(t *testing.T) {
	svc, r, _, _ := newTestService(t)
	r.orders["o-1"] = domain.Order{ID: "o-1", Code: "SB-1", Status: domain.StatusPaid}

	ev := paymentEvent("payment.proof_uploaded", map[string]any{"order_id": "o-1"})
	if err := svc.HandlePaymentEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if r.orders["o-1"].Status != domain.StatusPaid {
		t.Fatalf("status tidak boleh berubah: %s", r.orders["o-1"].Status)
	}
	if len(r.transitionEvents) != 0 {
		t.Fatalf("tidak boleh ada event: %+v", r.transitionEvents)
	}
}

func TestHandlePaymentEventIgnoresUnknownAndBadPayload(t *testing.T) {
	svc, _, _, _ := newTestService(t)

	if err := svc.HandlePaymentEvent(context.Background(), paymentEvent("payment.foo", map[string]any{"order_id": "o-1"})); err != nil {
		t.Fatalf("tipe lain harus diabaikan: %v", err)
	}

	bad := kafkax.Event{EventID: "e", Type: "payment.confirmed", Payload: json.RawMessage(`not-json`)}
	if err := svc.HandlePaymentEvent(context.Background(), bad); err == nil {
		t.Fatal("payload rusak harus error")
	}

	missing := paymentEvent("payment.confirmed", map[string]any{})
	if err := svc.HandlePaymentEvent(context.Background(), missing); err == nil {
		t.Fatal("order_id kosong harus error")
	}
}
