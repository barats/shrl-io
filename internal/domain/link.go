package domain

import "time"

// Link is the core entity: a Code on a Hostname that redirects to a
// Destination, created by a Creator (a User). A Link belongs to exactly one
// Team or is Personal (TeamID nil); its Team is fixed even if its Creator
// leaves or is removed from that Team.
type Link struct {
	Hostname    string    `json:"hostname" gorm:"primaryKey"`
	Code        string    `json:"code" gorm:"primaryKey"`
	Destination string    `json:"destination"`
	Remark      string    `json:"remark"`
	Disabled    bool      `json:"disabled"`
	CreatedBy   int64     `json:"created_by" gorm:"index"`
	TeamID      *int64    `json:"team_id" gorm:"index"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
