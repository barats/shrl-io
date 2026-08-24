package store

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// HostnameStore is the Postgres model for the Hostname Registry.
type HostnameStore struct {
	db *gorm.DB
}

func NewHostnameStore(db *gorm.DB) *HostnameStore { return &HostnameStore{db: db} }

func (s *HostnameStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&domain.Hostname{})
}

// Create registers a hostname. ErrDuplicatedKey if already registered.
func (s *HostnameStore) Create(ctx context.Context, h *domain.Hostname) error {
	err := s.db.WithContext(ctx).Create(h).Error
	if isDuplicateKey(err) {
		return ErrDuplicatedKey
	}
	return err
}

// Get returns the registered hostname, or ErrNotFound.
func (s *HostnameStore) Get(ctx context.Context, name string) (*domain.Hostname, error) {
	var h domain.Hostname
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// List returns every registered hostname, sorted.
func (s *HostnameStore) List(ctx context.Context) ([]domain.Hostname, error) {
	var hs []domain.Hostname
	err := s.db.WithContext(ctx).Order("name ASC").Find(&hs).Error
	return hs, err
}

func (s *HostnameStore) Delete(ctx context.Context, name string) error {
	return s.db.WithContext(ctx).Where("name = ?", name).Delete(&domain.Hostname{}).Error
}
