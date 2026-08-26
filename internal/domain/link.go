package domain

import "time"

// Link is the core entity: a globally unique Code that redirects to a
// Destination, served under the Hostname its Creator selected when creating
// it. A Link belongs to exactly one Team or is Personal (TeamID nil); its Team
// is fixed even if its Creator leaves or is removed from that Team.
type Link struct {
	Code        string    `json:"code" gorm:"primaryKey"`
	Hostname    string    `json:"hostname"`
	Destination string    `json:"destination"`
	Remark      string    `json:"remark"`
	Disabled    bool      `json:"disabled"`
	// ForwardUTM appends the six recognized UTM parameters from a visitor's
	// short URL to the Destination on Redirect. Off by default.
	ForwardUTM bool      `json:"forward_utm"`
	CreatedBy   int64     `json:"created_by" gorm:"index"`
	TeamID      *int64    `json:"team_id" gorm:"index"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
