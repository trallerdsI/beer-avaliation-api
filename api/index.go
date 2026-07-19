package handler

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"beer-review-app/pkg/vercel"
)

var (
	once    sync.Once
	handler http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		// Logger JSON para stdout garante visibilidade nos logs da Vercel.
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)
		slog.Info("vercel cold start: a construir handler")
		handler = vercel.NewHandler()
		slog.Info("vercel handler construído", "handler_nil", handler == nil)
	})

	if handler == nil {
		http.Error(w, "server initialization failed", http.StatusInternalServerError)
		return
	}

	handler.ServeHTTP(w, r)
}
