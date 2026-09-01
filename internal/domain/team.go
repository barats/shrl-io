package domain

import "time"

// RefLength is the length of every generated team Ref (ADR 0021).
const RefLength = 10

// Team is a group of Users created by an Admin. The Team is the ownership
// boundary for Links assigned to it: every Team Member sees all of the Team's
// Links and their related data.
type Team struct {
	ID int64 `json:"-" gorm:"primaryKey"`
	// Ref is the team's opaque external identifier (ADR 0021): the only team
	// id exposed in URLs and JSON, marshaled as "id". ID keys rows and
	// foreign keys internally and never leaves the database.
	Ref       string    `json:"id" gorm:"size:10"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:128"`
	CreatedBy int64     `json:"-" gorm:"index"`
	CreatedAt time.Time `json:"created_at"`
}

// GenerateRef returns a random Ref from the unambiguous Code alphabet.
func GenerateRef() (string, error) {
	return GenerateCode(RefLength)
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
