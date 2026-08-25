package store

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// CreateKey stores a new API key hash for a user.
func (s *UserStore) CreateKey(ctx context.Context, k *domain.APIKey) error {
	return s.db.WithContext(ctx).Create(k).Error
}

// KeyByHash returns the API key with the given SHA-256 hash, or ErrNotFound.
func (s *UserStore) KeyByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	var k domain.APIKey
	err := s.db.WithContext(ctx).Where("hash = ?", hash).First(&k).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListKeys returns a user's API keys, newest first (id breaks timestamp ties
// created within the same transaction instant).
func (s *UserStore) ListKeys(ctx context.Context, userID int64) ([]domain.APIKey, error) {
	var keys []domain.APIKey
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC, id DESC").Find(&keys).Error
	return keys, err
}

// DeleteKey removes an API key only if it belongs to the given user, so a user
// can never revoke someone else's key.
func (s *UserStore) DeleteKey(ctx context.Context, id, userID int64) error {
	res := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&domain.APIKey{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteKeysForUser removes every API key owned by the user.
func (s *UserStore) DeleteKeysForUser(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.APIKey{}).Error
}
