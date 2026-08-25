package domain

import "time"

// Setting is a single runtime-configurable instance setting, keyed by name.
// Settings are the first live knob in an otherwise env-configured instance
// (ADR 0013).
type Setting struct {
	Key       string    `json:"key" gorm:"primaryKey;size:64"`
	Value     string    `json:"value" gorm:"size:255"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CodeLengthSetting is the key of the per-instance Code Length setting.
const CodeLengthSetting = "code_length"
