// Package service berisi usecase ongkir: daftar zona, kuotasi, dan CRUD zona.
package service

import (
	"context"
	"errors"
	"strings"

	"saybu/pkg/ids"
	"saybu/services/shipping/internal/domain"
)

// Repo adalah kebutuhan service terhadap database.
type Repo interface {
	ListZones(ctx context.Context, includeInactive bool) ([]domain.Zone, error)
	GetZone(ctx context.Context, id string) (domain.Zone, error)
	CreateZone(ctx context.Context, z domain.Zone) (domain.Zone, error)
	UpdateZone(ctx context.Context, z domain.Zone) (domain.Zone, error)
	DeleteZone(ctx context.Context, id string) error
}

// Service adalah usecase ongkir.
type Service struct {
	repo Repo
}

// New membuat Service.
func New(repo Repo) (*Service, error) {
	if repo == nil {
		return nil, errors.New("service: repo wajib")
	}
	return &Service{repo: repo}, nil
}

// ListZones mengambil daftar zona.
func (s *Service) ListZones(ctx context.Context, includeInactive bool) ([]domain.Zone, error) {
	return s.repo.ListZones(ctx, includeInactive)
}

// Quote menghitung ongkir untuk sebuah zona dan subtotal.
func (s *Service) Quote(ctx context.Context, zoneID string, subtotal int64) (domain.Quote, error) {
	ve := &domain.ValidationError{}
	if strings.TrimSpace(zoneID) == "" {
		ve.Add("zone_id", "ID zona wajib diisi.")
	}
	if subtotal < 0 {
		ve.Add("subtotal", "Subtotal tidak boleh negatif.")
	}
	if err := ve.OrNil(); err != nil {
		return domain.Quote{}, err
	}

	zone, err := s.repo.GetZone(ctx, strings.TrimSpace(zoneID))
	if err != nil {
		return domain.Quote{}, err
	}
	if !zone.IsActive {
		return domain.Quote{}, domain.ErrZoneNotFound
	}
	return domain.ComputeQuote(zone, subtotal), nil
}

// UpsertZoneInput adalah input pembuatan/pembaruan zona.
type UpsertZoneInput struct {
	ID              string
	Name            string
	Areas           []string
	Fee             int64
	FreeMinSubtotal int64
	IsActive        *bool
}

// UpsertZone membuat zona baru (ID kosong) atau memperbarui yang ada.
func (s *Service) UpsertZone(ctx context.Context, in UpsertZoneInput) (domain.Zone, error) {
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	clean, err := domain.ValidateZone(domain.ZoneInput{
		ID:              in.ID,
		Name:            in.Name,
		Areas:           in.Areas,
		Fee:             in.Fee,
		FreeMinSubtotal: in.FreeMinSubtotal,
		IsActive:        active,
	})
	if err != nil {
		return domain.Zone{}, err
	}

	if strings.TrimSpace(in.ID) == "" {
		return s.repo.CreateZone(ctx, domain.Zone{
			ID:              ids.New(),
			Name:            clean.Name,
			Areas:           clean.Areas,
			Fee:             clean.Fee,
			FreeMinSubtotal: clean.FreeMinSubtotal,
			IsActive:        clean.IsActive,
		})
	}

	existing, err := s.repo.GetZone(ctx, strings.TrimSpace(in.ID))
	if err != nil {
		return domain.Zone{}, err
	}
	existing.Name = clean.Name
	existing.Areas = clean.Areas
	existing.Fee = clean.Fee
	existing.FreeMinSubtotal = clean.FreeMinSubtotal
	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}
	return s.repo.UpdateZone(ctx, existing)
}

// DeleteZone menghapus zona.
func (s *Service) DeleteZone(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return (&domain.ValidationError{}).Add("id", "ID zona wajib diisi.")
	}
	return s.repo.DeleteZone(ctx, strings.TrimSpace(id))
}
