package model

import "time"

type RefreshToken struct {
	ID        string `json:"-"`
	UserID    string `json:"-"`
	TokenHash string `json:"-"`
	ExpiresAt string `json:"-"`
	Revoked   bool   `json:"-"`
	CreatedAt string `json:"-"`
}

func (r RefreshToken) IsExpired() bool {
	if r.ExpiresAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, r.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(t)
}
