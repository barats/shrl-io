package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// ErrInvalidInvite is returned when an invite code is unknown or already used.
var ErrInvalidInvite = errors.New("invalid or already-used invite code")

// InviteStore is the Postgres model for team invite codes.
type InviteStore struct {
	db *gorm.DB
}

func NewInviteStore(db *gorm.DB) *InviteStore { return &InviteStore{db: db} }

func (s *InviteStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&domain.InviteCode{})
}

// Create stores a new invite code.
func (s *InviteStore) Create(ctx context.Context, inv *domain.InviteCode) error {
	err := s.db.WithContext(ctx).Create(inv).Error
	if isDuplicateKey(err) {
		return ErrDuplicatedKey
	}
	return err
}

// ListOutstanding returns a team's unused invite codes, newest first.
func (s *InviteStore) ListOutstanding(ctx context.Context, teamID int64) ([]domain.InviteCode, error) {
	var invites []domain.InviteCode
	err := s.db.WithContext(ctx).
		Where("team_id = ? AND used_at IS NULL", teamID).
		Order("created_at DESC").Find(&invites).Error
	return invites, err
}

// Revoke deletes an outstanding invite code. ok is false when no matching
// unused code existed.
func (s *InviteStore) Revoke(ctx context.Context, teamID int64, code string) (bool, error) {
	res := s.db.WithContext(ctx).
		Where("team_id = ? AND code = ? AND used_at IS NULL", teamID, code).
		Delete(&domain.InviteCode{})
	return res.RowsAffected > 0, res.Error
}

// JoinByCode consumes an invite code and adds the user to the team as a
// member, atomically. Codes are single-use: the first join claims the code, so
// a second user presenting it gets ErrInvalidInvite. Returns the team id.
// ErrInvalidInvite when the code is unknown or already used; ErrDuplicatedKey
// when the user is already a member (the code is left unconsumed).
func (s *InviteStore) JoinByCode(ctx context.Context, code string, userID int64) (int64, error) {
	var teamID int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv domain.InviteCode
		if err := tx.Where("code = ? AND used_at IS NULL", code).First(&inv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidInvite
			}
			return err
		}
		teamID = inv.TeamID
		var n int64
		if err := tx.Model(&domain.TeamMember{}).
			Where("team_id = ? AND user_id = ?", teamID, userID).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return ErrDuplicatedKey
		}
		// The conditional update is the atomic claim: only one joiner can win.
		res := tx.Model(&domain.InviteCode{}).
			Where("id = ? AND used_at IS NULL", inv.ID).
			Updates(map[string]any{"used_by": userID, "used_at": time.Now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidInvite
		}
		if err := tx.Create(&domain.TeamMember{TeamID: teamID, UserID: userID, Role: domain.RoleMember}).Error; err != nil {
			return err
		}
		return nil
	})
	return teamID, err
}
