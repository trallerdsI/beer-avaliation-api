package handler

import (
	"log/slog"
	"net/http"
	"os"

	"beer-review-app/internal/app"
	"beer-review-app/pkg/logging"
)

var router http.Handler

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, logging.SanitizeOptions(&slog.HandlerOptions{Level: slog.LevelInfo})))
	db, _ := app.InitDBFromEnv()
	if db != nil {
		router = app.BuildRouter(db, logger)
	} else {
		router = app.BuildRouter(nil, logger)
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	router.ServeHTTP(w, r)
}
