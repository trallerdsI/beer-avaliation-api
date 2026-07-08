package handler

import (
	"net/http"
	"sync"

	"beer-review-app/pkg/vercel"
)

var (
	once    sync.Once
	handler http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		handler = vercel.NewHandler()
	})

	if handler == nil {
		http.Error(w, "server initialization failed", http.StatusInternalServerError)
		return
	}

	handler.ServeHTTP(w, r)
}
