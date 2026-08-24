package domain

import "time"

// Link is the core entity: a Code on a Hostname that redirects to a
// Destination, created by a Creator (a User).
type Link struct {
	Hostname    string    `json:"hostname" gorm:"primaryKey"`
	Code        string    `json:"code" gorm:"primaryKey"`
	Destination string    `json:"destination"`
	Remark      string    `json:"remark"`
	Disabled    bool      `json:"disabled"`
	CreatedBy   int64     `json:"created_by" gorm:"index"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
