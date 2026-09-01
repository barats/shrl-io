package store

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// BaseURLStore is the Postgres model for the Base URL Registry.
type BaseURLStore struct {
	db *gorm.DB
}

func NewBaseURLStore(db *gorm.DB) *BaseURLStore { return &BaseURLStore{db: db} }

func (s *BaseURLStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&domain.BaseURL{})
}

// Create registers a base URL. ErrDuplicatedKey if already registered.
func (s *BaseURLStore) Create(ctx context.Context, b *domain.BaseURL) error {
	err := s.db.WithContext(ctx).Create(b).Error
	if isDuplicateKey(err) {
		return ErrDuplicatedKey
	}
	return err
}

// Get returns the registered base URL, or ErrNotFound.
func (s *BaseURLStore) Get(ctx context.Context, baseURL string) (*domain.BaseURL, error) {
	var b domain.BaseURL
	err := s.db.WithContext(ctx).Where("base_url = ?", baseURL).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// List returns every registered base URL, sorted.
func (s *BaseURLStore) List(ctx context.Context) ([]domain.BaseURL, error) {
	var bs []domain.BaseURL
	err := s.db.WithContext(ctx).Order("base_url ASC").Find(&bs).Error
	return bs, err
}

func (s *BaseURLStore) Delete(ctx context.Context, baseURL string) error {
	return s.db.WithContext(ctx).Where("base_url = ?", baseURL).Delete(&domain.BaseURL{}).Error
}
