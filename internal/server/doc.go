// Package server provides an HTTP server for exposing Prometheus metrics.
//
// This package implements an HTTP server with multiple endpoints for
// serving Prometheus metrics, health checks, and a web UI. It provides
// graceful shutdown support and configurable timeouts.
//
// Available endpoints:
//   - /           : Web UI showing exporter status and information
//   - /metrics    : Prometheus metrics endpoint
//   - /health     : Liveness probe (always returns 200)
//   - /ready      : Readiness probe (returns 200 only when data is loaded)
//
// The server is configured with sensible timeout defaults:
//   - Read timeout: 15 seconds
//   - Write timeout: 15 seconds
//   - Idle timeout: 60 seconds
//
// The main type is Server, which manages the HTTP server lifecycle and
// provides methods for starting and graceful shutdown.
//
// Example usage:
//
//	// Create server
//	srv := server.NewServer(cfg, collector)
//
//	// Start server in a goroutine
//	serverErrors := make(chan error, 1)
//	go func() {
//		serverErrors <- srv.Start()
//	}()
//
//	// Wait for shutdown signal
//	shutdown := make(chan os.Signal, 1)
//	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
//
//	select {
//	case err := <-serverErrors:
//		log.Fatalf("Server error: %v", err)
//	case <-shutdown:
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//		if err := srv.Shutdown(ctx); err != nil {
//			log.Printf("Error during shutdown: %v", err)
//		}
//	}
//
// The web UI provides a user-friendly interface showing:
//   - Current exporter status (Ready/Not Ready)
//   - Last scrape time
//   - Number of cost records cached
//   - Refresh interval
//   - Number of subscriptions monitored
//   - Links to all available endpoints
package server
