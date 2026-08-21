package model

import "time"

// BeerEvent represents a domain event for a beer entity.
type BeerEvent struct {
	Type      string         `json:"type"`
	ID        string         `json:"id"`
	BeerID    string         `json:"beerId"`
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
}
