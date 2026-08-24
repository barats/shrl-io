package store

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// ErrNotFound is returned when a link does not exist.
var ErrNotFound = errors.New("not found")

// Store is the Postgres write model via GORM. Dialect-neutral: the same
// model and queries work with the MySQL driver.
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(
		&domain.Link{},
		&domain.DailyStats{},
		&domain.Breakdown{},
		&domain.LifetimeStats{},
	)
}

func (s *Store) Create(ctx context.Context, l *domain.Link) error {
	return s.db.WithContext(ctx).Create(l).Error
}

func (s *Store) Get(ctx context.Context, hostname, code string) (*domain.Link, error) {
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

func (s *Store) List(ctx context.Context, hostname string) ([]domain.Link, error) {
	var links []domain.Link
	err := s.db.WithContext(ctx).Where("hostname = ?", hostname).
		Order("created_at DESC").Find(&links).Error
	return links, err
}

// ListActive returns all non-disabled links, for the cache warmer.
func (s *Store) ListActive(ctx context.Context) ([]domain.Link, error) {
	var links []domain.Link
	err := s.db.WithContext(ctx).Where("disabled = false").Find(&links).Error
	return links, err
}

func (s *Store) Save(ctx context.Context, l *domain.Link) error {
	return s.db.WithContext(ctx).Save(l).Error
}

func (s *Store) Delete(ctx context.Context, hostname, code string) error {
	return s.db.WithContext(ctx).
		Where("hostname = ? AND code = ?", hostname, code).
		Delete(&domain.Link{}).Error
}
