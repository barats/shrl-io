package store

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/barats/shrl-io/internal/domain"
)

// TeamStore is the Postgres model for teams and their memberships.
type TeamStore struct {
	db *gorm.DB
}

func NewTeamStore(db *gorm.DB) *TeamStore { return &TeamStore{db: db} }

func (s *TeamStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&domain.Team{}, &domain.TeamMember{})
}

func (s *TeamStore) Create(ctx context.Context, t *domain.Team) error {
	err := s.db.WithContext(ctx).Create(t).Error
	if isDuplicateKey(err) {
		return ErrDuplicatedKey
	}
	return err
}

func (s *TeamStore) Get(ctx context.Context, id int64) (*domain.Team, error) {
	var t domain.Team
	err := s.db.WithContext(ctx).First(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListForUser returns the teams a user belongs to, oldest first.
func (s *TeamStore) ListForUser(ctx context.Context, userID int64) ([]domain.Team, error) {
	var teams []domain.Team
	err := s.db.WithContext(ctx).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("team_members.user_id = ?", userID).
		Order("teams.created_at ASC").Find(&teams).Error
	return teams, err
}

func (s *TeamStore) ListAll(ctx context.Context) ([]domain.Team, error) {
	var teams []domain.Team
	err := s.db.WithContext(ctx).Order("created_at ASC").Find(&teams).Error
	return teams, err
}

// AddMember adds a user to a team with a role.
func (s *TeamStore) AddMember(ctx context.Context, teamID, userID int64, role domain.TeamRole) error {
	m := &domain.TeamMember{TeamID: teamID, UserID: userID, Role: role}
	err := s.db.WithContext(ctx).Create(m).Error
	if isDuplicateKey(err) {
		return ErrDuplicatedKey
	}
	return err
}

// MemberRole returns the user's role in the team, or ErrNotFound.
func (s *TeamStore) MemberRole(ctx context.Context, teamID, userID int64) (domain.TeamRole, error) {
	var m domain.TeamMember
	err := s.db.WithContext(ctx).Where("team_id = ? AND user_id = ?", teamID, userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return m.Role, nil
}

// ListMembers returns a team's memberships with roles, oldest join first.
func (s *TeamStore) ListMembers(ctx context.Context, teamID int64) ([]domain.TeamMember, error) {
	var members []domain.TeamMember
	err := s.db.WithContext(ctx).Where("team_id = ?", teamID).Order("joined_at ASC").Find(&members).Error
	return members, err
}

// CountOwners returns the number of owners in the team.
func (s *TeamStore) CountOwners(ctx context.Context, teamID int64) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&domain.TeamMember{}).
		Where("team_id = ? AND role = ?", teamID, domain.RoleOwner).Count(&n).Error
	return n, err
}

// SetRole updates a member's role in the team.
func (s *TeamStore) SetRole(ctx context.Context, teamID, userID int64, role domain.TeamRole) error {
	return s.db.WithContext(ctx).Model(&domain.TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Update("role", role).Error
}

// RemoveMember removes a user from a team. ok is false when the user was not
// a member.
func (s *TeamStore) RemoveMember(ctx context.Context, teamID, userID int64) (bool, error) {
	res := s.db.WithContext(ctx).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Delete(&domain.TeamMember{})
	return res.RowsAffected > 0, res.Error
}

// Delete removes a team and all its memberships.
func (s *TeamStore) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("team_id = ?", id).Delete(&domain.TeamMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.Team{}, id).Error
	})
}
