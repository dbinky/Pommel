package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pommel-dev/pommel/internal/config"
	"github.com/pommel-dev/pommel/internal/daemon"
)

// Router wraps a chi router with handler configuration
type Router struct {
	chi     chi.Router
	handler *Handler
	logger  *slog.Logger
}

// NewRouter creates a new Router with the given dependencies
func NewRouter(indexer *daemon.Indexer, cfg *config.Config, searcher Searcher) *Router {
	handler := NewHandler(indexer, cfg, searcher)

	r := chi.NewRouter()

	// Apply middleware
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.Recoverer)

	// Register routes
	r.Get("/health", handler.Health)
	r.Get("/status", handler.Status)
	r.Post("/search", handler.Search)
	r.Post("/reindex", handler.Reindex)
	r.Get("/config", handler.Config)

	return &Router{
		chi:     r,
		handler: handler,
		logger:  nil,
	}
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.chi.ServeHTTP(w, req)
}
