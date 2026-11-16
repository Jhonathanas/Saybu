package domain

import "testing"

func TestListOrdersFilterNormalize(t *testing.T) {
	f := ListOrdersFilter{Status: " PAID ", Query: " budi ", Page: 0, Limit: 0}
	f.Normalize()
	if f.Status != "PAID" || f.Query != "budi" {
		t.Fatalf("filter tidak dirapikan: %+v", f)
	}
	if f.Page != 1 || f.Limit != 20 {
		t.Fatalf("halaman tidak dinormalkan: %+v", f)
	}
	f.Limit = 500
	f.Normalize()
	if f.Limit != 100 {
		t.Fatalf("limit tidak dibatasi: %d", f.Limit)
	}
}

func TestListOrdersFilterValidate(t *testing.T) {
	cases := []struct {
		name    string
		filter  ListOrdersFilter
		wantErr bool
	}{
		{"kosong", ListOrdersFilter{}, false},
		{"status sah", ListOrdersFilter{Status: StatusPaid}, false},
		{"status salah", ListOrdersFilter{Status: "NOPE"}, true},
		{"tanggal sah", ListOrdersFilter{DateFrom: "2026-11-01", DateTo: "2026-11-30"}, false},
		{"tanggal salah", ListOrdersFilter{DateFrom: "01-11-2026"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.filter.Validate()
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}
