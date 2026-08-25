package store

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// ErrNotFound is returned when a link does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicatedKey is returned when a link with the same hostname+code already
// exists. It wraps the driver's unique-violation error.
var ErrDuplicatedKey = errors.New("duplicate key")

// isDuplicateKey reports whether err is a unique-constraint violation. GORM
// maps Postgres/MySQL driver errors to gorm.ErrDuplicatedKey; sqlite surfaces
// them as raw constraint errors, so match the message too.
func isDuplicateKey(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate entry")
}

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
	if isDuplicateKey(err) {
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

// List returns the personal links (not assigned to a team) of one creator on
// a hostname, newest first.
func (s *LinkStore) List(ctx context.Context, hostname string, creatorID int64) ([]domain.Link, error) {
	var links []domain.Link
	err := s.db.WithContext(ctx).
		Where("hostname = ? AND created_by = ? AND team_id IS NULL", hostname, creatorID).
		Order("created_at DESC").Find(&links).Error
	return links, err
}

// ListByTeam returns the links of a team on a hostname, newest first.
func (s *LinkStore) ListByTeam(ctx context.Context, hostname string, teamID int64) ([]domain.Link, error) {
	var links []domain.Link
	err := s.db.WithContext(ctx).
		Where("hostname = ? AND team_id = ?", hostname, teamID).
		Order("created_at DESC").Find(&links).Error
	return links, err
}

// TransferTeamLinksToPersonal moves every link of a team to Personal scope,
// keeping each link's creator. Used when a team is deleted; the team is the
// only exception to the no-transfer rule.
func (s *LinkStore) TransferTeamLinksToPersonal(ctx context.Context, teamID int64) error {
	return s.db.WithContext(ctx).Model(&domain.Link{}).
		Where("team_id = ?", teamID).
		Update("team_id", gorm.Expr("NULL")).Error
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

// ListPersonalByCreator returns every Personal Link a user created, across all
// hostnames. Used to evict a deleted user's links from the redirect cache.
func (s *LinkStore) ListPersonalByCreator(ctx context.Context, creatorID int64) ([]domain.Link, error) {
	var links []domain.Link
	err := s.db.WithContext(ctx).
		Where("created_by = ? AND team_id IS NULL", creatorID).
		Find(&links).Error
	return links, err
}
