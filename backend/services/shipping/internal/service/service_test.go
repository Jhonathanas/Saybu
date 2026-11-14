package service

import (
	"context"
	"errors"
	"testing"

	"saybu/services/shipping/internal/domain"
)

type fakeRepo struct {
	zones   map[string]domain.Zone
	created []domain.Zone
	updated []domain.Zone
	deleted []string
}

func newFake() *fakeRepo {
	return &fakeRepo{zones: map[string]domain.Zone{
		"z1": {ID: "z1", Name: "Kec. Sukamaju", Fee: 10000, FreeMinSubtotal: 150000, IsActive: true},
		"z2": {ID: "z2", Name: "Zona Nonaktif", Fee: 5000, IsActive: false},
	}}
}

func (f *fakeRepo) ListZones(_ context.Context, includeInactive bool) ([]domain.Zone, error) {
	var out []domain.Zone
	for _, z := range f.zones {
		if includeInactive || z.IsActive {
			out = append(out, z)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetZone(_ context.Context, id string) (domain.Zone, error) {
	z, ok := f.zones[id]
	if !ok {
		return domain.Zone{}, domain.ErrZoneNotFound
	}
	return z, nil
}

func (f *fakeRepo) CreateZone(_ context.Context, z domain.Zone) (domain.Zone, error) {
	for _, ex := range f.zones {
		if ex.Name == z.Name {
			return domain.Zone{}, domain.ErrZoneExists
		}
	}
	f.zones[z.ID] = z
	f.created = append(f.created, z)
	return z, nil
}

func (f *fakeRepo) UpdateZone(_ context.Context, z domain.Zone) (domain.Zone, error) {
	if _, ok := f.zones[z.ID]; !ok {
		return domain.Zone{}, domain.ErrZoneNotFound
	}
	f.zones[z.ID] = z
	f.updated = append(f.updated, z)
	return z, nil
}

func (f *fakeRepo) DeleteZone(_ context.Context, id string) error {
	if _, ok := f.zones[id]; !ok {
		return domain.ErrZoneNotFound
	}
	delete(f.zones, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func newService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	r := newFake()
	s, err := New(r)
	if err != nil {
		t.Fatal(err)
	}
	return s, r
}

func TestQuote(t *testing.T) {
	s, _ := newService(t)
	q, err := s.Quote(context.Background(), "z1", 75000)
	if err != nil {
		t.Fatal(err)
	}
	if q.Fee != 10000 || q.Total != 85000 || q.ZoneName != "Kec. Sukamaju" {
		t.Fatalf("quote = %+v", q)
	}
}

func TestQuoteFreeShipping(t *testing.T) {
	s, _ := newService(t)
	q, err := s.Quote(context.Background(), "z1", 150000)
	if err != nil {
		t.Fatal(err)
	}
	if !q.FreeShipping || q.Fee != 0 || q.Total != 150000 {
		t.Fatalf("quote = %+v", q)
	}
}

func TestQuoteUnknownZone(t *testing.T) {
	s, _ := newService(t)
	_, err := s.Quote(context.Background(), "nope", 1000)
	if !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestQuoteInactiveZoneTreatedAsNotFound(t *testing.T) {
	s, _ := newService(t)
	_, err := s.Quote(context.Background(), "z2", 1000)
	if !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestQuoteValidation(t *testing.T) {
	s, _ := newService(t)
	_, err := s.Quote(context.Background(), "", -1)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestCreateZone(t *testing.T) {
	s, r := newService(t)
	z, err := s.UpsertZone(context.Background(), UpsertZoneInput{
		Name: "Kec. Baru", Areas: []string{"Baru"}, Fee: 9000, FreeMinSubtotal: 100000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if z.ID == "" || !z.IsActive || z.Fee != 9000 {
		t.Fatalf("zone = %+v", z)
	}
	if len(r.created) != 1 {
		t.Fatalf("created = %+v", r.created)
	}
}

func TestCreateZoneDuplicateName(t *testing.T) {
	s, _ := newService(t)
	_, err := s.UpsertZone(context.Background(), UpsertZoneInput{Name: "Kec. Sukamaju", Fee: 1})
	if !errors.Is(err, domain.ErrZoneExists) {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateZoneKeepsActiveWhenOmitted(t *testing.T) {
	s, r := newService(t)
	z, err := s.UpsertZone(context.Background(), UpsertZoneInput{
		ID: "z1", Name: "Kec. Sukamaju Baru", Fee: 12000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if z.Name != "Kec. Sukamaju Baru" || z.Fee != 12000 {
		t.Fatalf("zone = %+v", z)
	}
	if !r.zones["z1"].IsActive {
		t.Fatal("is_active harus tetap true bila tidak dikirim")
	}
}

func TestUpdateZoneDeactivate(t *testing.T) {
	s, r := newService(t)
	off := false
	_, err := s.UpsertZone(context.Background(), UpsertZoneInput{ID: "z1", Name: "Kec. Sukamaju", Fee: 10000, IsActive: &off})
	if err != nil {
		t.Fatal(err)
	}
	if r.zones["z1"].IsActive {
		t.Fatal("zona harus nonaktif")
	}
}

func TestUpdateZoneNotFound(t *testing.T) {
	s, _ := newService(t)
	_, err := s.UpsertZone(context.Background(), UpsertZoneInput{ID: "nope", Name: "Zona X", Fee: 1})
	if !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteZone(t *testing.T) {
	s, r := newService(t)
	if err := s.DeleteZone(context.Background(), "z1"); err != nil {
		t.Fatal(err)
	}
	if len(r.deleted) != 1 {
		t.Fatalf("deleted = %+v", r.deleted)
	}
	if err := s.DeleteZone(context.Background(), "z1"); !errors.Is(err, domain.ErrZoneNotFound) {
		t.Fatalf("err = %v", err)
	}
}
