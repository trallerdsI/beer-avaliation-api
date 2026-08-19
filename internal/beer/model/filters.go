package model

type BeerFilters struct {
	Query      string   `json:"query"`
	Style      string   `json:"style"`
	MinAlcohol *float64 `json:"minAlcohol"`
	MaxAlcohol *float64 `json:"maxAlcohol"`
	Taste      string   `json:"taste"`
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
	Fuzzy      *bool    `json:"fuzzy,omitempty"`
}
