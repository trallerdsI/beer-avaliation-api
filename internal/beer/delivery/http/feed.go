package http

import (
	"net/http"

	"beer-review-app/pkg/response"
)

// HomeFeedItem é a projeção enxuta de uma cerveja para a tela inicial do app
// móvel (sem descrições longas nem comentários embutidos completos).
type HomeFeedItem struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Style     string   `json:"style"`
	ImageURL  string   `json:"imageUrl,omitempty"`
	Alcohol   *float64 `json:"alcohol,omitempty"`
	CommentCount int   `json:"commentCount"`
}

// HomeFeed é o payload único (BFF) que monta a tela inicial com uma requisição.
type HomeFeed struct {
	Featured []HomeFeedItem `json:"featured"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

// GetHomeFeed consolida em um único payload JSON o necessário para renderizar
// a home do app móvel, eliminando N requests REST sequenciais (BFF Pattern).
// Campos pesados (description, comments[]) são omitidos via projeção explícita.
func (c *BeerController) GetHomeFeed(w http.ResponseWriter, r *http.Request) {
	page := getIntParam(r.URL.Query(), "page", 1)
	pageSize := getIntParam(r.URL.Query(), "pageSize", 20)

	beers, total, err := c.usecase.GetPaginated(r.Context(), page, pageSize)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to load home feed", http.StatusInternalServerError)
		return
	}

	items := make([]HomeFeedItem, 0, len(beers))
	for _, b := range beers {
		items = append(items, HomeFeedItem{
			ID:          b.ID,
			Name:        b.Name,
			Style:       b.Style,
			ImageURL:    b.ImageUrl,
			Alcohol:     b.Alcohol,
			CommentCount: len(b.Comments),
		})
	}

	response.SendResponse(w, http.StatusOK, HomeFeed{
		Featured: items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}