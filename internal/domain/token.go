package domain

import "time"

// Token is a bearer credential issued at Login, stored as a SHA-256 hash so a
// database leak never exposes usable tokens. Revocable at Logout.
type Token struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	UserID    int64     `json:"user_id" gorm:"index"`
	Hash      string    `json:"-" gorm:"uniqueIndex;size:64"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
