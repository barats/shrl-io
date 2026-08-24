package domain

import "time"

// User is an account that signs in and manages Links.
type User struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64"`
	PasswordHash string    `json:"-" gorm:"size:255"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
}
