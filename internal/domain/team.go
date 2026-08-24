package domain

import "time"

// Team is a group of Users created by an Admin. The Team is the ownership
// boundary for Links assigned to it: every Team Member sees all of the Team's
// Links and their related data.
type Team struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:128"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// TeamRole is the role a User holds in a Team.
type TeamRole string

const (
	RoleOwner  TeamRole = "owner"
	RoleMember TeamRole = "member"
)

// TeamMember links a User to a Team with a role.
type TeamMember struct {
	TeamID   int64     `json:"team_id" gorm:"primaryKey"`
	UserID   int64     `json:"user_id" gorm:"primaryKey"`
	Role     TeamRole  `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}
