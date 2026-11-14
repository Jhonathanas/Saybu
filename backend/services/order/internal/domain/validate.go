package domain

import "strings"

// ValidateCreate memvalidasi dan menormalkan permintaan checkout.
//
// Validasi ini tidak menyentuh katalog: kecocokan harga, stok, dan langkah qty
// diverifikasi di usecase setelah produk diambil dari catalog service.
func ValidateCreate(in CreateInput) (CreateInput, error) {
	errs := &ValidationError{}

	name := strings.TrimSpace(in.Customer.Name)
	if name == "" {
		errs.Add("customer.name", "Nama pembeli wajib diisi.")
	} else if len(name) > 120 {
		errs.Add("customer.name", "Nama pembeli terlalu panjang.")
	}

	phone := NormalizePhone(in.Customer.Phone)
	if phone == "" {
		errs.Add("customer.phone", "Nomor HP wajib diisi.")
	} else if len(phone) < 8 || len(phone) > 15 {
		errs.Add("customer.phone", "Nomor HP tidak valid.")
	}

	email := strings.TrimSpace(in.Customer.Email)
	if email != "" && !strings.Contains(email, "@") {
		errs.Add("customer.email", "Email tidak valid.")
	}

	fulfillment := strings.ToUpper(strings.TrimSpace(in.Fulfillment))
	switch fulfillment {
	case FulfillmentDelivery, FulfillmentPickup:
	default:
		errs.Add("fulfillment", "Pemenuhan harus DELIVERY atau PICKUP.")
	}

	method := strings.ToUpper(strings.TrimSpace(in.PaymentMethod))
	switch method {
	case PaymentTransfer, PaymentCOD:
	default:
		errs.Add("payment_method", "Metode pembayaran harus TRANSFER atau COD.")
	}

	address := Address{
		Line:       strings.TrimSpace(in.Address.Line),
		RTRW:       strings.TrimSpace(in.Address.RTRW),
		Kelurahan:  strings.TrimSpace(in.Address.Kelurahan),
		Kecamatan:  strings.TrimSpace(in.Address.Kecamatan),
		City:       strings.TrimSpace(in.Address.City),
		PostalCode: strings.TrimSpace(in.Address.PostalCode),
		Landmark:   strings.TrimSpace(in.Address.Landmark),
	}
	zoneID := strings.TrimSpace(in.ZoneID)
	if fulfillment == FulfillmentDelivery {
		if zoneID == "" {
			errs.Add("zone_id", "Zona ongkir wajib untuk pengiriman.")
		}
		if address.Line == "" {
			errs.Add("address.line", "Alamat wajib untuk pengiriman.")
		}
	}

	if len(in.Items) == 0 {
		errs.Add("items", "Pesanan harus berisi minimal satu produk.")
	}
	items := make([]ItemInput, 0, len(in.Items))
	seen := make(map[string]bool, len(in.Items))
	for _, it := range in.Items {
		pid := strings.TrimSpace(it.ProductID)
		if pid == "" {
			errs.Add("items.product_id", "ID produk wajib diisi.")
			continue
		}
		if it.Qty <= 0 {
			errs.Add("items.qty", "Jumlah produk harus lebih dari nol.")
			continue
		}
		if seen[pid] {
			// Gabungkan qty produk yang sama agar tidak ada baris ganda.
			for j := range items {
				if items[j].ProductID == pid {
					items[j].Qty += it.Qty
					break
				}
			}
			continue
		}
		seen[pid] = true
		items = append(items, ItemInput{ProductID: pid, Qty: it.Qty})
	}

	if err := errs.OrNil(); err != nil {
		return CreateInput{}, err
	}

	if fulfillment == FulfillmentPickup {
		zoneID = ""
		address = Address{}
	}

	return CreateInput{
		Customer: Customer{
			Name:  name,
			Phone: phone,
			Email: email,
			Note:  strings.TrimSpace(in.Customer.Note),
		},
		Fulfillment:   fulfillment,
		ZoneID:        zoneID,
		Address:       address,
		PaymentMethod: method,
		Items:         items,
		UserID:        strings.TrimSpace(in.UserID),
	}, nil
}

// CancellableByCustomer melaporkan apakah pembeli boleh membatalkan pada status ini.
func CancellableByCustomer(status string) bool { return cancellableByCustomer(status) }
