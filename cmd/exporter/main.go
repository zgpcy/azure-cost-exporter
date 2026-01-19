package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zgpcy/azure-cost-exporter/internal/azure"
	"github.com/zgpcy/azure-cost-exporter/internal/collector"
	"github.com/zgpcy/azure-cost-exporter/internal/config"
	"github.com/zgpcy/azure-cost-exporter/internal/logger"
	"github.com/zgpcy/azure-cost-exporter/internal/server"
)

const (
	// DefaultShutdownTimeout is the maximum time to wait for graceful shutdown
	DefaultShutdownTimeout = 30 * time.Second
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	version    = "dev"
)

func main() {
	flag.Parse()

	// Load configuration first (need log level from config)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize structured logger
	logger := logger.New(cfg.LogLevel)
	logger.Info("Azure Cost Exporter starting",
		"version", version,
		"config_path", *configPath)

	logger.Info("Configuration loaded successfully",
		"subscriptions", len(cfg.Subscriptions),
		"refresh_interval_seconds", cfg.RefreshInterval,
		"http_port", cfg.HTTPPort,
		"days_to_query", cfg.DateRange.DaysToQuery,
		"end_date_offset", cfg.DateRange.EndDateOffset,
		"currency", cfg.Currency,
		"grouping_enabled", cfg.GroupBy.Enabled,
		"api_timeout_seconds", cfg.APITimeout)

	if cfg.GroupBy.Enabled {
		logger.Info("Grouping configuration",
			"dimensions", len(cfg.GroupBy.Groups))
	}

	// Create Azure client
	logger.Info("Initializing Azure Cost Management client")
	azureClient, err := azure.NewClient(cfg, logger)
	if err != nil {
		logger.Error("Failed to create Azure client", "error", err)
		os.Exit(1)
	}
	logger.Info("Azure client initialized successfully")

	// Create cost collector
	logger.Info("Creating Prometheus collector")
	costCollector := collector.NewCostCollector(azureClient, cfg, logger)

	// Register collector with Prometheus
	if err := prometheus.Register(costCollector); err != nil {
		logger.Error("Failed to register collector", "error", err)
		os.Exit(1)
	}
	logger.Info("Collector registered with Prometheus")

	// Register Go runtime metrics (memory, goroutines, GC stats)
	if err := prometheus.Register(prometheus.NewGoCollector()); err != nil {
		logger.Warn("Failed to register Go collector", "error", err)
	} else {
		logger.Info("Go runtime metrics registered")
	}

	// Register process metrics (CPU, memory, file descriptors)
	if err := prometheus.Register(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{})); err != nil {
		logger.Warn("Failed to register process collector", "error", err)
	} else {
		logger.Info("Process metrics registered")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background refresh
	logger.Info("Starting background cost data refresh")
	costCollector.StartBackgroundRefresh(ctx)

	// Create and start HTTP server
	logger.Info("Creating HTTP server", "port", cfg.HTTPPort)
	srv := server.NewServer(cfg, costCollector, logger)

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- srv.Start()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("Server error", "error", err)
		os.Exit(1)

	case sig := <-shutdown:
		logger.Info("Received shutdown signal, starting graceful shutdown", "signal", sig.String())

		// Cancel background refresh
		cancel()

		// Shutdown server with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("Error during server shutdown", "error", err)
			// Force shutdown
			os.Exit(1)
		}

		logger.Info("Server stopped gracefully")
	}
}
