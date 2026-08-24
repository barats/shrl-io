package store

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// UserStore is the Postgres model for accounts and bearer tokens.
type UserStore struct {
	db *gorm.DB
}

func NewUserStore(db *gorm.DB) *UserStore { return &UserStore{db: db} }

func (s *UserStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&domain.User{}, &domain.Token{})
}

func (s *UserStore) Count(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&domain.User{}).Count(&n).Error
	return n, err
}

func (s *UserStore) Create(ctx context.Context, u *domain.User) error {
	err := s.db.WithContext(ctx).Create(u).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicatedKey
	}
	return err
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	err := s.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	err := s.db.WithContext(ctx).First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) List(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	err := s.db.WithContext(ctx).Order("created_at ASC").Find(&users).Error
	return users, err
}

func (s *UserStore) CreateToken(ctx context.Context, t *domain.Token) error {
	return s.db.WithContext(ctx).Create(t).Error
}

// TokenByHash returns the token with the given SHA-256 hash, or ErrNotFound.
func (s *UserStore) TokenByHash(ctx context.Context, hash string) (*domain.Token, error) {
	var t domain.Token
	err := s.db.WithContext(ctx).Where("hash = ?", hash).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *UserStore) DeleteToken(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&domain.Token{}, id).Error
}

// AssignLinksToCreator backfills links with no creator to the given user,
// used at first-run bootstrap for pre-existing links.
func (s *UserStore) AssignLinksToCreator(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).
		Model(&domain.Link{}).
		Where("created_by IS NULL OR created_by = 0").
		Update("created_by", userID).Error
}
