// Package service berisi usecase pesanan: saga checkout, pelacakan, pembatalan,
// dan job kedaluwarsa.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	catalogv1 "saybu/gen/saybu/catalog/v1"
	shippingv1 "saybu/gen/saybu/shipping/v1"
	"saybu/pkg/ids"
	"saybu/pkg/kafkax"
	"saybu/pkg/outbox"
	"saybu/services/order/internal/domain"
	"saybu/services/order/internal/repo"
)

// Nama topik dan producer untuk event pesanan.
const (
	TopicOrderEvents = "order.events.v1"
	producerName     = "order-svc"
)

// Repo adalah kebutuhan service terhadap database.
type Repo interface {
	GetIdempotency(ctx context.Context, key string) (repo.Idempotency, bool, error)
	GetOrderByCode(ctx context.Context, code string) (domain.Order, error)
	GetOrderByID(ctx context.Context, id string) (domain.Order, error)
	ListOrdersByUser(ctx context.Context, userID string, page, limit int) ([]domain.Order, int64, error)
	ListOrders(ctx context.Context, f domain.ListOrdersFilter) ([]domain.Order, int64, error)
	TodayStats(ctx context.Context) (domain.TodayStats, error)
	CreateOrder(ctx context.Context, o domain.Order, idem repo.Idempotency, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, error)
	Transition(ctx context.Context, id, from, to, note, actor string, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, error)
	TransitionFromPayment(ctx context.Context, eventID, eventType, orderID, from, to, note, actor string, events func(domain.Order) ([]outbox.Record, error)) (domain.Order, bool, error)
	ExpireDue(ctx context.Context, limit int, events func(domain.Order) ([]outbox.Record, error)) ([]domain.Order, error)
}

// CatalogClient adalah bagian klien gRPC katalog yang dipakai order service.
type CatalogClient interface {
	GetProduct(ctx context.Context, in *catalogv1.GetProductRequest, opts ...grpc.CallOption) (*catalogv1.Product, error)
	CheckStock(ctx context.Context, in *catalogv1.CheckStockRequest, opts ...grpc.CallOption) (*catalogv1.CheckStockResponse, error)
	ReserveStock(ctx context.Context, in *catalogv1.ReserveStockRequest, opts ...grpc.CallOption) (*catalogv1.ReserveStockResponse, error)
	ReleaseStock(ctx context.Context, in *catalogv1.ReleaseStockRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

// ShippingClient adalah bagian klien gRPC ongkir yang dipakai order service.
type ShippingClient interface {
	Quote(ctx context.Context, in *shippingv1.QuoteRequest, opts ...grpc.CallOption) (*shippingv1.QuoteResponse, error)
}

// Service adalah usecase pesanan.
type Service struct {
	repo     Repo
	catalog  CatalogClient
	shipping ShippingClient
	log      *slog.Logger
	expiry   time.Duration
	now      func() time.Time
}

// New membuat Service.
func New(r Repo, catalog CatalogClient, shipping ShippingClient, log *slog.Logger, expiry time.Duration) (*Service, error) {
	if r == nil {
		return nil, errors.New("service: repo wajib")
	}
	if catalog == nil {
		return nil, errors.New("service: klien katalog wajib")
	}
	if shipping == nil {
		return nil, errors.New("service: klien ongkir wajib")
	}
	if log == nil {
		log = slog.Default()
	}
	if expiry <= 0 {
		expiry = 24 * time.Hour
	}
	return &Service{repo: r, catalog: catalog, shipping: shipping, log: log, expiry: expiry, now: time.Now}, nil
}

// CreateOrder menjalankan saga checkout.
//
// Urutan: validasi -> idempotensi (fast path) -> ambil produk & hitung harga ->
// kuotasi ongkir -> cek stok -> reservasi stok -> simpan pesanan + outbox.
// Bila penyimpanan gagal setelah reservasi, stok dilepas (kompensasi).
func (s *Service) CreateOrder(ctx context.Context, in domain.CreateInput, idempotencyKey string) (domain.Order, error) {
	clean, err := domain.ValidateCreate(in)
	if err != nil {
		return domain.Order{}, err
	}

	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return domain.Order{}, domain.ErrIdempotencyKeyMissing
	}
	hash := requestHash(clean)

	if existing, found, err := s.repo.GetIdempotency(ctx, key); err != nil {
		return domain.Order{}, err
	} else if found {
		if existing.RequestHash != hash {
			return domain.Order{}, domain.ErrIdempotencyMismatch
		}
		return s.repo.GetOrderByID(ctx, existing.OrderID)
	}

	items, subtotal, err := s.buildItems(ctx, clean.Items)
	if err != nil {
		return domain.Order{}, err
	}

	fee, zoneName, err := s.quote(ctx, clean, subtotal)
	if err != nil {
		return domain.Order{}, err
	}

	stockItems := toStockItems(items)
	check, err := s.catalog.CheckStock(ctx, &catalogv1.CheckStockRequest{Items: stockItems})
	if err != nil {
		return domain.Order{}, err
	}
	if !check.GetAllOk() {
		return domain.Order{}, insufficientFromProto(check.GetItems())
	}

	orderID := ids.New()
	res, err := s.catalog.ReserveStock(ctx, &catalogv1.ReserveStockRequest{OrderId: orderID, Items: stockItems})
	if err != nil {
		return domain.Order{}, err
	}
	if !res.GetReserved() {
		return domain.Order{}, insufficientFromProto(res.GetInsufficient())
	}

	order := domain.Order{
		ID:            orderID,
		UserID:        clean.UserID,
		Fulfillment:   clean.Fulfillment,
		PaymentMethod: clean.PaymentMethod,
		Customer:      clean.Customer,
		Address:       clean.Address,
		ZoneID:        clean.ZoneID,
		ZoneName:      zoneName,
		ShippingFee:   fee,
		Subtotal:      subtotal,
		Total:         subtotal + fee,
		Items:         items,
		ExpiresAt:     s.now().UTC().Add(s.expiry),
	}

	created, err := s.repo.CreateOrder(ctx, order, repo.Idempotency{Key: key, RequestHash: hash},
		func(o domain.Order) ([]outbox.Record, error) { return s.orderCreatedEvent(o) })
	if err != nil {
		s.compensate(ctx, orderID)
		if errors.Is(err, domain.ErrIdempotencyExists) {
			return s.replayExisting(ctx, key, hash)
		}
		return domain.Order{}, err
	}
	return created, nil
}

// GetOrderByCode mengambil pesanan untuk pelacakan (guest via HP, akun via user).
func (s *Service) GetOrderByCode(ctx context.Context, code, phone, userID string) (domain.Order, error) {
	o, err := s.repo.GetOrderByCode(ctx, code)
	if err != nil {
		return domain.Order{}, err
	}
	if err := authorize(o, phone, userID); err != nil {
		return domain.Order{}, err
	}
	return o, nil
}

// ListMyOrders mengambil daftar pesanan milik user.
func (s *Service) ListMyOrders(ctx context.Context, userID string, page, limit int) ([]domain.Order, int64, int, int, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, 0, 0, 0, (&domain.ValidationError{}).Add("user_id", "Pengguna wajib login.")
	}
	page, limit = normalizePage(page, limit)
	orders, total, err := s.repo.ListOrdersByUser(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return orders, total, page, limit, nil
}

// GetOrderByID mengambil pesanan berdasarkan id untuk panel admin.
func (s *Service) GetOrderByID(ctx context.Context, id string) (domain.Order, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Order{}, (&domain.ValidationError{}).Add("id", "ID pesanan wajib diisi.")
	}
	return s.repo.GetOrderByID(ctx, id)
}

// ListOrders mengambil daftar pesanan untuk admin dengan filter & paginasi.
func (s *Service) ListOrders(ctx context.Context, f domain.ListOrdersFilter) ([]domain.Order, int64, int, int, error) {
	f.Normalize()
	if err := f.Validate(); err != nil {
		return nil, 0, 0, 0, err
	}
	orders, total, err := s.repo.ListOrders(ctx, f)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return orders, total, f.Page, f.Limit, nil
}

// UpdateStatus memindahkan status pesanan oleh admin. Transisi divalidasi
// terhadap status saat ini; transisi ilegal ditolak (ErrInvalidTransition).
func (s *Service) UpdateStatus(ctx context.Context, id, to, note, actor string) (domain.Order, error) {
	id = strings.TrimSpace(id)
	to = strings.TrimSpace(to)
	if id == "" {
		return domain.Order{}, (&domain.ValidationError{}).Add("id", "ID pesanan wajib diisi.")
	}
	if !domain.IsValidStatus(to) {
		return domain.Order{}, (&domain.ValidationError{}).Add("status", "Status tidak dikenal.")
	}

	o, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	if !domain.CanTransition(o.Status, to) {
		return domain.Order{}, domain.ErrInvalidTransition
	}

	note = strings.TrimSpace(note)
	actor = actorOr(actor, "admin")
	from := o.Status
	return s.repo.Transition(ctx, id, from, to, note, actor,
		func(ord domain.Order) ([]outbox.Record, error) {
			return s.statusChangedEvent(ord, from, to, actor, note)
		})
}

// GetTodayStats mengambil ringkasan operasional hari ini untuk dashboard admin.
func (s *Service) GetTodayStats(ctx context.Context) (domain.TodayStats, error) {
	return s.repo.TodayStats(ctx)
}

// CancelOrder membatalkan pesanan pembeli selama status mengizinkan.
func (s *Service) CancelOrder(ctx context.Context, code, phone, userID, reason string) (domain.Order, error) {
	o, err := s.repo.GetOrderByCode(ctx, code)
	if err != nil {
		return domain.Order{}, err
	}
	if err := authorize(o, phone, userID); err != nil {
		return domain.Order{}, err
	}
	if !domain.CancellableByCustomer(o.Status) {
		return domain.Order{}, domain.ErrCancelNotAllowed
	}
	reason = strings.TrimSpace(reason)
	return s.repo.Transition(ctx, o.ID, o.Status, domain.StatusCancelled, reason, "customer",
		func(ord domain.Order) ([]outbox.Record, error) { return s.orderCancelledEvent(ord, reason) })
}

// ExpireDue menandai pesanan kedaluwarsa dan menulis event order.expired.
// Mengembalikan jumlah pesanan yang kedaluwarsa.
func (s *Service) ExpireDue(ctx context.Context) (int, error) {
	orders, err := s.repo.ExpireDue(ctx, 100, func(o domain.Order) ([]outbox.Record, error) {
		return s.orderExpiredEvent(o)
	})
	if err != nil {
		return 0, err
	}
	return len(orders), nil
}

// RunExpiry menjalankan job kedaluwarsa berkala sampai context dibatalkan.
func (s *Service) RunExpiry(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if n, err := s.ExpireDue(ctx); err != nil {
			if ctx.Err() == nil {
				s.log.Error("job expire order gagal", "error", err)
			}
		} else if n > 0 {
			s.log.Info("order kedaluwarsa", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// buildItems mengambil harga & satuan dari katalog lalu menghitung total baris.
func (s *Service) buildItems(ctx context.Context, inputs []domain.ItemInput) ([]domain.Item, int64, error) {
	items := make([]domain.Item, 0, len(inputs))
	var subtotal int64
	for _, in := range inputs {
		p, err := s.catalog.GetProduct(ctx, &catalogv1.GetProductRequest{Id: in.ProductID})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil, 0, (&domain.ValidationError{}).Add("items", "Produk tidak ditemukan atau tidak aktif: "+in.ProductID)
			}
			return nil, 0, err
		}

		step := p.GetStepQty()
		if step <= 0 {
			step = 1
		}
		min := p.GetMinQty()
		if min <= 0 {
			min = step
		}
		if in.Qty < min {
			return nil, 0, (&domain.ValidationError{}).
				Add("items", fmt.Sprintf("Jumlah %s minimal %d.", p.GetName(), min))
		}
		if in.Qty%step != 0 {
			return nil, 0, (&domain.ValidationError{}).
				Add("items", fmt.Sprintf("Jumlah %s harus kelipatan %d.", p.GetName(), step))
		}

		line := domain.LineTotal(p.GetUnit(), p.GetPrice(), in.Qty)
		subtotal += line
		items = append(items, domain.Item{
			ProductID: p.GetId(),
			Name:      p.GetName(),
			Unit:      p.GetUnit(),
			Qty:       in.Qty,
			UnitPrice: p.GetPrice(),
			LineTotal: line,
		})
	}
	return items, subtotal, nil
}

// quote menghitung ongkir. PICKUP selalu gratis dan mengabaikan zona.
func (s *Service) quote(ctx context.Context, in domain.CreateInput, subtotal int64) (int64, string, error) {
	if in.Fulfillment == domain.FulfillmentPickup {
		return 0, "", nil
	}
	res, err := s.shipping.Quote(ctx, &shippingv1.QuoteRequest{ZoneId: in.ZoneID, Subtotal: subtotal})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition || status.Code(err) == codes.NotFound {
			return 0, "", &domain.ZoneNotFoundError{ZoneID: in.ZoneID}
		}
		return 0, "", err
	}
	return res.GetFee(), res.GetZoneName(), nil
}

// replayExisting mengembalikan pesanan yang sudah tersimpan untuk kunci idempotensi.
func (s *Service) replayExisting(ctx context.Context, key, hash string) (domain.Order, error) {
	existing, found, err := s.repo.GetIdempotency(ctx, key)
	if err != nil {
		return domain.Order{}, err
	}
	if !found {
		return domain.Order{}, fmt.Errorf("service: kunci idempotensi hilang setelah konflik")
	}
	if existing.RequestHash != hash {
		return domain.Order{}, domain.ErrIdempotencyMismatch
	}
	return s.repo.GetOrderByID(ctx, existing.OrderID)
}

// compensate melepas reservasi stok bila penyimpanan pesanan gagal.
func (s *Service) compensate(ctx context.Context, orderID string) {
	if _, err := s.catalog.ReleaseStock(ctx, &catalogv1.ReleaseStockRequest{OrderId: orderID}); err != nil {
		s.log.Error("kompensasi release stok gagal", "order_id", orderID, "error", err)
	}
}

func authorize(o domain.Order, phone, userID string) error {
	if userID != "" && o.UserID != "" && o.UserID == userID {
		return nil
	}
	if phone != "" && domain.PhoneMatches(o.Customer.Phone, phone) {
		return nil
	}
	// Jangan membocorkan keberadaan pesanan milik orang lain.
	return domain.ErrOrderNotFound
}

func toStockItems(items []domain.Item) []*catalogv1.StockItem {
	out := make([]*catalogv1.StockItem, 0, len(items))
	for _, it := range items {
		out = append(out, &catalogv1.StockItem{ProductId: it.ProductID, Qty: it.Qty})
	}
	return out
}

func insufficientFromProto(items []*catalogv1.StockStatus) error {
	short := make([]domain.StockShort, 0, len(items))
	for _, it := range items {
		short = append(short, domain.StockShort{
			ProductID: it.GetProductId(),
			Name:      it.GetName(),
			Requested: it.GetRequested(),
			Available: it.GetAvailable(),
		})
	}
	return &domain.InsufficientStockError{Items: short}
}

func requestHash(in domain.CreateInput) string {
	canonical := struct {
		Fulfillment   string
		ZoneID        string
		PaymentMethod string
		Customer      domain.Customer
		Address       domain.Address
		Items         []domain.ItemInput
	}{
		Fulfillment:   in.Fulfillment,
		ZoneID:        in.ZoneID,
		PaymentMethod: in.PaymentMethod,
		Customer:      in.Customer,
		Address:       in.Address,
		Items:         in.Items,
	}
	raw, _ := json.Marshal(canonical)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Service) orderCreatedEvent(o domain.Order) ([]outbox.Record, error) {
	items := make([]map[string]any, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, map[string]any{
			"product_id": it.ProductID,
			"name":       it.Name,
			"unit":       it.Unit,
			"qty":        it.Qty,
			"unit_price": it.UnitPrice,
		})
	}
	payload := map[string]any{
		"order_id":       o.ID,
		"code":           o.Code,
		"user_id":        o.UserID,
		"customer":       map[string]any{"name": o.Customer.Name, "phone": o.Customer.Phone},
		"fulfillment":    o.Fulfillment,
		"payment_method": o.PaymentMethod,
		"zone_name":      o.ZoneName,
		"address_line":   o.Address.Line,
		"subtotal":       o.Subtotal,
		"shipping_fee":   o.ShippingFee,
		"total":          o.Total,
		"items":          items,
		"expires_at":     o.ExpiresAt.Format(time.RFC3339),
	}
	rec, err := newRecord("order.created", o.ID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func (s *Service) orderCancelledEvent(o domain.Order, reason string) ([]outbox.Record, error) {
	payload := map[string]any{
		"order_id": o.ID,
		"code":     o.Code,
		"reason":   reason,
		"by":       "customer",
		"items":    itemRefs(o.Items),
	}
	rec, err := newRecord("order.cancelled", o.ID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func (s *Service) orderExpiredEvent(o domain.Order) ([]outbox.Record, error) {
	payload := map[string]any{
		"order_id": o.ID,
		"code":     o.Code,
		"items":    itemRefs(o.Items),
	}
	rec, err := newRecord("order.expired", o.ID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func itemRefs(items []domain.Item) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{"product_id": it.ProductID, "qty": it.Qty})
	}
	return out
}

// HandlePaymentEvent memproses event dari payment.events.v1 dan memindahkan
// status pesanan sesuai hasil pembayaran. Idempotent lewat processed_events.
func (s *Service) HandlePaymentEvent(ctx context.Context, ev kafkax.Event) error {
	var p struct {
		OrderID    string `json:"order_id"`
		Code       string `json:"code"`
		Reason     string `json:"reason"`
		VerifiedBy string `json:"verified_by"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("service: payload %s tidak valid: %w", ev.Type, err)
	}
	if strings.TrimSpace(p.OrderID) == "" {
		return fmt.Errorf("service: event %s tanpa order_id", ev.Type)
	}

	var from, to, note, actor string
	switch ev.Type {
	case "payment.proof_uploaded":
		from = domain.StatusPendingPayment
		to = domain.StatusWaitingVerification
		note = "Bukti transfer diunggah"
		actor = "customer"
	case "payment.confirmed":
		from = domain.StatusWaitingVerification
		to = domain.StatusPaid
		note = "Pembayaran diverifikasi"
		actor = actorOr(p.VerifiedBy, "admin")
	case "payment.rejected":
		from = domain.StatusWaitingVerification
		to = domain.StatusPendingPayment
		note = "Bukti transfer ditolak"
		if strings.TrimSpace(p.Reason) != "" {
			note += ": " + strings.TrimSpace(p.Reason)
		}
		actor = actorOr(p.VerifiedBy, "admin")
	default:
		return nil
	}

	order, applied, err := s.repo.TransitionFromPayment(ctx, ev.EventID, ev.Type, p.OrderID,
		from, to, note, actor,
		func(o domain.Order) ([]outbox.Record, error) {
			return s.statusChangedEvent(o, from, to, actor, note)
		})
	if err != nil {
		return err
	}
	if !applied {
		s.log.Debug("transisi dari event pembayaran tidak diterapkan",
			"type", ev.Type, "order_id", p.OrderID, "from", from)
		return nil
	}
	s.log.Info("status pesanan berubah dari event pembayaran",
		"type", ev.Type, "code", order.Code, "from", from, "to", to)
	return nil
}

func (s *Service) statusChangedEvent(o domain.Order, from, to, actor, note string) ([]outbox.Record, error) {
	payload := map[string]any{
		"order_id": o.ID,
		"code":     o.Code,
		"from":     from,
		"to":       to,
		"by":       actor,
		"note":     note,
	}
	rec, err := newRecord("order.status_changed", o.ID, payload)
	if err != nil {
		return nil, err
	}
	return []outbox.Record{rec}, nil
}

func actorOr(actor, def string) string {
	if strings.TrimSpace(actor) == "" {
		return def
	}
	return actor
}

func newRecord(eventType, key string, payload any) (outbox.Record, error) {
	ev, err := kafkax.NewEvent(eventType, key, payload, producerName)
	if err != nil {
		return outbox.Record{}, fmt.Errorf("service: bangun event %s: %w", eventType, err)
	}
	raw, err := ev.Marshal()
	if err != nil {
		return outbox.Record{}, fmt.Errorf("service: marshal event %s: %w", eventType, err)
	}
	return outbox.Record{
		EventID: ev.EventID,
		Topic:   TopicOrderEvents,
		Key:     key,
		Payload: raw,
		Headers: map[string]string{"event-type": eventType},
	}, nil
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
