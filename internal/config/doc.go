// Package config provides configuration management for the Azure Cost Exporter.
//
// This package handles loading configuration from YAML files, applying
// environment variable overrides, setting defaults, and validating the
// configuration.
//
// Configuration sources (in order of precedence):
//   1. Environment variables (highest priority)
//   2. YAML configuration file
//   3. Default values (lowest priority)
//
// Supported environment variables:
//   - AZURE_COST_CURRENCY: Currency symbol for cost values
//   - AZURE_COST_REFRESH_INTERVAL: Refresh interval in seconds (minimum: 60)
//   - AZURE_COST_HTTP_PORT: HTTP server port (1-65535)
//   - AZURE_COST_LOG_LEVEL: Log level (debug, info, warn, error)
//   - AZURE_COST_END_DATE_OFFSET: Days to offset the end date
//   - AZURE_COST_DAYS_TO_QUERY: Number of days to query (minimum: 1)
//   - AZURE_COST_SUBSCRIPTIONS: Comma-separated subscription IDs or id:name pairs
//
// The main type is Config, which contains all application settings including:
//   - Subscriptions: List of Azure subscriptions to monitor
//   - DateRange: Date range configuration for cost queries
//   - GroupBy: Grouping configuration for cost queries
//   - RefreshInterval: How often to refresh cost data
//   - HTTPPort: Port for the HTTP server
//   - LogLevel: Logging verbosity
//   - Currency: Currency symbol to use in metrics
//
// Example configuration file (config.yaml):
//
//	subscriptions:
//	  - id: "sub-123"
//	    name: "Production"
//	  - id: "sub-456"
//	    name: "Development"
//
//	currency: "€"
//	refresh_interval: 3600  # 1 hour
//	http_port: 8080
//	log_level: "info"
//
//	date_range:
//	  end_date_offset: 1    # Yesterday
//	  days_to_query: 7      # Last 7 days
//
//	group_by:
//	  enabled: true
//	  groups:
//	    - type: "Dimension"
//	      name: "ResourceGroup"
//	      label_name: "resource_group"
//
// Example usage:
//
//	cfg, err := config.Load("config.yaml")
//	if err != nil {
//		log.Fatalf("Failed to load config: %v", err)
//	}
//
//	fmt.Printf("Monitoring %d subscriptions\n", len(cfg.Subscriptions))
//	fmt.Printf("Refresh interval: %d seconds\n", cfg.RefreshInterval)
package config
