package vercel

import (
	"net/http"

	"beer-review-app/internal/app"
)

// NewHandler creates a handler that boots the shared Vercel-compatible application.
func NewHandler() http.Handler {
	return app.InitializeVercelHandler()
}
