package store

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// ErrNotFound is returned when a link does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicatedKey is returned when a link with the same hostname+code already
// exists. It wraps the driver's unique-violation error.
var ErrDuplicatedKey = errors.New("duplicate key")

// LinkStore is the Postgres write model for links via GORM. Dialect-neutral:
// the same model and queries work with the MySQL driver.
type LinkStore struct {
	db *gorm.DB
}

func NewLinkStore(db *gorm.DB) *LinkStore { return &LinkStore{db: db} }

func (s *LinkStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&domain.Link{})
}

func (s *LinkStore) Create(ctx context.Context, l *domain.Link) error {
	err := s.db.WithContext(ctx).Create(l).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicatedKey
	}
	return err
}

func (s *LinkStore) Get(ctx context.Context, hostname, code string) (*domain.Link, error) {
	var l domain.Link
	err := s.db.WithContext(ctx).Where("hostname = ? AND code = ?", hostname, code).First(&l).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *LinkStore) List(ctx context.Context, hostname string) ([]domain.Link, error) {
	var links []domain.Link
	err := s.db.WithContext(ctx).Where("hostname = ?", hostname).
		Order("created_at DESC").Find(&links).Error
	return links, err
}

// ListActive returns all non-disabled links, for the cache warmer.
func (s *LinkStore) ListActive(ctx context.Context) ([]domain.Link, error) {
	var links []domain.Link
	err := s.db.WithContext(ctx).Where("disabled = false").Find(&links).Error
	return links, err
}

func (s *LinkStore) Save(ctx context.Context, l *domain.Link) error {
	return s.db.WithContext(ctx).Save(l).Error
}

func (s *LinkStore) Delete(ctx context.Context, hostname, code string) error {
	return s.db.WithContext(ctx).
		Where("hostname = ? AND code = ?", hostname, code).
		Delete(&domain.Link{}).Error
}
