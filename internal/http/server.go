package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/logging"
)

// Server represents the HTTP server
type Server struct {
	config *config.Config
	logger *logging.Logger
	server *http.Server
	deps   *HandlerDependencies
}

// New creates a new HTTP server
func New(cfg *config.Config, logger *logging.Logger, deps *HandlerDependencies) *Server {
	return &Server{
		config: cfg,
		logger: logger,
		deps:   deps,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Create router
	r := chi.NewRouter()

	// Apply middleware
	r.Use(CORS)
	r.Use(RequestSizeLimit(10 * 1024 * 1024)) // 10MB limit
	r.Use(RequestTimeout(90 * time.Second))
	r.Use(LoggingMiddleware(s.logger))

	// Public routes
	r.Get("/health", HandleHealth(s.deps))

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(s.config, s.logger))
		r.Post("/v1/chat/completions", HandleChatCompletions(s.deps))
		r.Post("/v1/messages", HandleMessages(s.deps))
		r.Post("/v1/messages/count_tokens", HandleMessagesCountTokens(s.deps))
		r.Get("/v1/models", HandleModels(s.deps))
	})

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.logger.Info("Starting HTTP server", map[string]any{
		"host": s.config.Host,
		"port": s.config.Port,
	})

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server", nil)
	return s.server.Shutdown(ctx)
}
