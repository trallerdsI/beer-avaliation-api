package model

type BeerFilters struct {
	Query      string   `json:"query"`
	Style      string   `json:"style"`
	MinAlcohol *float64 `json:"minAlcohol,omitempty"`
	MaxAlcohol *float64 `json:"maxAlcohol,omitempty"`
	Taste      string   `json:"taste"`
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
}
