package domain

import "time"

// Link is the core entity: a Code on a Hostname that redirects to a
// Destination.
type Link struct {
	Hostname    string    `json:"hostname" gorm:"primaryKey"`
	Code        string    `json:"code" gorm:"primaryKey"`
	Destination string    `json:"destination"`
	Disabled    bool      `json:"disabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
