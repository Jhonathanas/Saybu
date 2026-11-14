package domain

import (
	"errors"
	"testing"
)

func validInput() CreateInput {
	return CreateInput{
		Customer:      Customer{Name: "Siti", Phone: "081234567890"},
		Fulfillment:   FulfillmentDelivery,
		ZoneID:        "zone-1",
		Address:       Address{Line: "Jl. Melati 12"},
		PaymentMethod: PaymentTransfer,
		Items:         []ItemInput{{ProductID: "p1", Qty: 2}},
	}
}

func TestValidateCreateOK(t *testing.T) {
	in, err := ValidateCreate(validInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Customer.Phone != "081234567890" {
		t.Fatalf("phone tidak dinormalisasi: %q", in.Customer.Phone)
	}
	if in.Fulfillment != FulfillmentDelivery || in.ZoneID != "zone-1" {
		t.Fatalf("input tidak sesuai: %+v", in)
	}
}

func TestValidateCreatePickupClearsZone(t *testing.T) {
	in := validInput()
	in.Fulfillment = FulfillmentPickup
	in.ZoneID = "zone-1"
	in.Address = Address{Line: "Jl. Melati 12"}

	out, err := ValidateCreate(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ZoneID != "" || out.Address.Line != "" {
		t.Fatalf("pickup harus mengabaikan zona & alamat: %+v", out)
	}
}

func TestValidateCreateMergesDuplicateProducts(t *testing.T) {
	in := validInput()
	in.Items = []ItemInput{{ProductID: "p1", Qty: 2}, {ProductID: "p1", Qty: 3}}

	out, err := ValidateCreate(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].Qty != 5 {
		t.Fatalf("items harus digabung: %+v", out.Items)
	}
}

func TestValidateCreateErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateInput)
		field  string
	}{
		{"nama kosong", func(in *CreateInput) { in.Customer.Name = "  " }, "customer.name"},
		{"hp kosong", func(in *CreateInput) { in.Customer.Phone = "" }, "customer.phone"},
		{"hp pendek", func(in *CreateInput) { in.Customer.Phone = "123" }, "customer.phone"},
		{"email salah", func(in *CreateInput) { in.Customer.Email = "bukan-email" }, "customer.email"},
		{"fulfillment salah", func(in *CreateInput) { in.Fulfillment = "TERBANG" }, "fulfillment"},
		{"metode salah", func(in *CreateInput) { in.PaymentMethod = "QRIS" }, "payment_method"},
		{"zona kosong", func(in *CreateInput) { in.ZoneID = "" }, "zone_id"},
		{"alamat kosong", func(in *CreateInput) { in.Address = Address{} }, "address.line"},
		{"tanpa item", func(in *CreateInput) { in.Items = nil }, "items"},
		{"qty nol", func(in *CreateInput) { in.Items[0].Qty = 0 }, "items.qty"},
		{"produk kosong", func(in *CreateInput) { in.Items[0].ProductID = "" }, "items.product_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			_, err := ValidateCreate(in)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			found := false
			for _, fe := range ve.Errors {
				if fe.Field == tt.field {
					found = true
				}
			}
			if !found {
				t.Fatalf("field %q tidak dilaporkan: %+v", tt.field, ve.Errors)
			}
		})
	}
}
