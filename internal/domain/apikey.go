package domain

import "time"

// APIKey is a long-lived bearer credential a User creates, names, and revokes
// for programmatic access. Stored as a SHA-256 hash so a database leak never
// exposes usable keys; never expires; shown in full only at creation.
// Unscoped: a Key grants the same powers as its owner's Login.
type APIKey struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	UserID    int64     `json:"user_id" gorm:"index"`
	Name      string    `json:"name" gorm:"size:64"`
	Hash      string    `json:"-" gorm:"uniqueIndex;size:64"`
	CreatedAt time.Time `json:"created_at"`
}
