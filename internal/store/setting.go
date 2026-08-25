package store

import (
	"context"
	"errors"
	"strconv"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// SettingStore is the Postgres model for runtime-configurable instance settings.
type SettingStore struct {
	db *gorm.DB
}

func NewSettingStore(db *gorm.DB) *SettingStore { return &SettingStore{db: db} }

func (s *SettingStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&domain.Setting{})
}

// Has reports whether a setting with the given key exists.
func (s *SettingStore) Has(ctx context.Context, key string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&domain.Setting{}).Where("key = ?", key).Count(&n).Error
	return n > 0, err
}

// CodeLength returns the current Code Length, or DefaultCodeLength when unset.
func (s *SettingStore) CodeLength(ctx context.Context) (int, error) {
	var st domain.Setting
	err := s.db.WithContext(ctx).Where("key = ?", domain.CodeLengthSetting).First(&st).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.DefaultCodeLength, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(st.Value)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// SetCodeLength stores the Code Length, rejecting values outside the bounds.
func (s *SettingStore) SetCodeLength(ctx context.Context, n int) error {
	if err := domain.ValidateCodeLength(n); err != nil {
		return err
	}
	st := domain.Setting{Key: domain.CodeLengthSetting, Value: strconv.Itoa(n)}
	return s.db.WithContext(ctx).Save(&st).Error
}
