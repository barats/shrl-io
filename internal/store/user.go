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
	return s.db.WithContext(ctx).AutoMigrate(&domain.User{}, &domain.Token{}, &domain.APIKey{})
}

func (s *UserStore) Count(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&domain.User{}).Count(&n).Error
	return n, err
}

func (s *UserStore) Create(ctx context.Context, u *domain.User) error {
	err := s.db.WithContext(ctx).Create(u).Error
	if isDuplicateKey(err) {
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

// SetPassword replaces a user's password hash and clears the forced-change
// flag, completing a self-service change or an admin-issued reset.
func (s *UserStore) SetPassword(ctx context.Context, id int64, hash string) error {
	return s.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"password_hash": hash, "must_change_password": false}).Error
}

// RequirePasswordChange flags a user as needing a new password on next login
// (the temp password from an admin-issued reset).
func (s *UserStore) RequirePasswordChange(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", id).
		Update("must_change_password", true).Error
}

// DeleteTokensForUser removes every login token for a user.
func (s *UserStore) DeleteTokensForUser(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.Token{}).Error
}

// DeleteTokensForUserExcept removes every login token for a user except the
// one presented on the current request, so a password change keeps the
// current session alive while revoking everything else.
func (s *UserStore) DeleteTokensForUserExcept(ctx context.Context, userID int64, keepHash string) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND hash <> ?", userID, keepHash).
		Delete(&domain.Token{}).Error
}

// AssignLinksToCreator backfills links with no creator to the given user,
// used at first-run bootstrap for pre-existing links.
func (s *UserStore) AssignLinksToCreator(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).
		Model(&domain.Link{}).
		Where("created_by IS NULL OR created_by = 0").
		Update("created_by", userID).Error
}

// Delete removes a user and everything owned by them: bearer tokens, API
// keys, memberships, and Personal Links. Team Links they created stay with
// the Team (the fixed-team rule), leaving created_by as a dangling id.
func (s *UserStore) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&domain.Token{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&domain.APIKey{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&domain.TeamMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("created_by = ? AND team_id IS NULL", id).Delete(&domain.Link{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.User{}, id).Error
	})
}
