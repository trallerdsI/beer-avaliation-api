package handler

import (
	"net/http"
	"sync"

	"beer-review-app/internal/app"
)

var (
	once    sync.Once
	handler http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		handler = app.InitializeVercelHandler()
	})

	handler.ServeHTTP(w, r)
}
