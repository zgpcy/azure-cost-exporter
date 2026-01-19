package server

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zgpcy/azure-cost-exporter/internal/collector"
	"github.com/zgpcy/azure-cost-exporter/internal/config"
	"github.com/zgpcy/azure-cost-exporter/internal/logger"
)

//go:embed templates/index.html
var indexTemplate string

// HTTP server timeout constants
const (
	DefaultReadTimeout  = 15 * time.Second // Maximum duration for reading the entire request
	DefaultWriteTimeout = 15 * time.Second // Maximum duration before timing out writes of the response
	DefaultIdleTimeout  = 60 * time.Second // Maximum amount of time to wait for the next request
)

// indexPageData holds template data for the index page
type indexPageData struct {
	StatusClass       string
	StatusText        string
	LastScrape        string
	RecordCount       int
	RefreshInterval   int
	SubscriptionCount int
}

// Server represents the HTTP server
type Server struct {
	server    *http.Server
	collector *collector.CostCollector
	cfg       *config.Config
	logger    *logger.Logger
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, collector *collector.CostCollector, log *logger.Logger) *Server {
	mux := http.NewServeMux()

	s := &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
			Handler:      mux,
			ReadTimeout:  DefaultReadTimeout,
			WriteTimeout: DefaultWriteTimeout,
			IdleTimeout:  DefaultIdleTimeout,
		},
		collector: collector,
		cfg:       cfg,
		logger:    log,
	}

	// Register handlers
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.Handle("/metrics", promhttp.Handler())

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server", "address", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	return s.server.Shutdown(ctx)
}

// handleIndex serves a simple landing page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Parse template
	tmpl, err := template.New("index").Parse(indexTemplate)
	if err != nil {
		s.logger.Error("Failed to parse index template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	ready := s.collector.IsReady()
	statusClass := "not-ready"
	statusText := "Not Ready"
	if ready {
		statusClass = "ready"
		statusText = "Ready"
	}

	lastScrape := s.collector.LastScrapeTime()
	lastScrapeText := "Never"
	if !lastScrape.IsZero() {
		lastScrapeText = lastScrape.Format("2006-01-02 15:04:05 MST")
	}

	data := indexPageData{
		StatusClass:       statusClass,
		StatusText:        statusText,
		LastScrape:        lastScrapeText,
		RecordCount:       s.collector.RecordCount(),
		RefreshInterval:   s.cfg.RefreshInterval,
		SubscriptionCount: len(s.cfg.Subscriptions),
	}

	// Execute template
	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		s.logger.Error("Failed to execute index template", "error", err)
	}
}

// handleHealth handles health check requests (always returns 200 for liveness)
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"healthy"}`)); err != nil {
		s.logger.Error("Failed to write health response", "error", err)
	}
}

// handleReady handles readiness check requests (returns 200 only when data is loaded)
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !s.collector.IsReady() {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte(`{"status":"not ready","message":"waiting for initial data fetch"}`)); err != nil {
			s.logger.Error("Failed to write ready response", "error", err)
		}
		return
	}

	if err := s.collector.LastError(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, writeErr := fmt.Fprintf(w, `{"status":"not ready","error":"%s"}`, err.Error()); writeErr != nil {
			s.logger.Error("Failed to write ready response", "error", writeErr)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ready"}`)); err != nil {
		s.logger.Error("Failed to write ready response", "error", err)
	}
}
