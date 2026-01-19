// Package collector implements a Prometheus collector for cloud cost metrics.
//
// This package provides a Prometheus-compatible collector that periodically
// fetches cloud cost data from any provider and exposes it as metrics. It implements the
// prometheus.Collector interface and manages background refresh cycles.
//
// The collector exposes the following metrics:
//   - cloud_cost_daily: Daily cloud cost with comprehensive dimensions
//   - cloud_cost_exporter_up: Health status (1 = success, 0 = failure) with provider label
//   - cloud_cost_exporter_scrape_duration_seconds: Duration of the last scrape with provider label
//   - cloud_cost_exporter_scrape_errors_total: Total number of scrape errors with provider label
//   - cloud_cost_exporter_last_scrape_timestamp_seconds: Unix timestamp of last successful scrape with provider label
//   - cloud_cost_exporter_records_count: Number of cost records currently cached with provider label
//
// The main type is CostCollector, which:
//   - Fetches cost data from any cloud provider in the background at configurable intervals
//   - Caches the results to serve Prometheus scrapes quickly
//   - Provides thread-safe access to metrics via RWMutex
//   - Tracks operational metrics (scrape duration, errors, etc.)
//   - Works with any provider.CloudProvider implementation
//
// Example usage:
//
//	azureProvider, _ := azure.NewClient(cfg)
//	collector := collector.NewCostCollector(azureProvider, cfg)
//
//	// Register with Prometheus
//	prometheus.MustRegister(collector)
//
//	// Start background refresh
//	ctx := context.Background()
//	collector.StartBackgroundRefresh(ctx)
//
//	// Check readiness
//	if collector.IsReady() {
//		fmt.Println("Collector is ready")
//	}
package collector
