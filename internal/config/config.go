package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Configuration validation constants
const (
	MinRefreshInterval = 60    // Minimum refresh interval in seconds
	MinPort            = 1     // Minimum valid port number
	MaxPort            = 65535 // Maximum valid port number
	MinDaysToQuery     = 1     // Minimum days to query

	// Default values
	DefaultCurrency        = "€"
	DefaultEndDateOffset   = 1
	DefaultDaysToQuery     = 7
	DefaultRefreshInterval = 3600 // 1 hour in seconds
	DefaultHTTPPort        = 8080
	DefaultLogLevel        = "info"
	DefaultAPITimeout      = 30 // API timeout in seconds
)

// Subscription represents an Azure subscription to monitor
type Subscription struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// GroupBy represents grouping configuration for cost queries
type GroupBy struct {
	Type      string `yaml:"type"`
	Name      string `yaml:"name"`
	LabelName string `yaml:"label_name"`
}

// GroupByConfig represents the grouping configuration
type GroupByConfig struct {
	Enabled bool      `yaml:"enabled"`
	Groups  []GroupBy `yaml:"groups"`
}

// DateRange represents the date range configuration
type DateRange struct {
	EndDateOffset int `yaml:"end_date_offset"`
	DaysToQuery   int `yaml:"days_to_query"`
}

// Config represents the application configuration
type Config struct {
	Subscriptions                []Subscription `yaml:"subscriptions"`
	Currency                     string         `yaml:"currency"`
	DateRange                    DateRange      `yaml:"date_range"`
	GroupBy                      GroupByConfig  `yaml:"group_by"`
	RefreshInterval              int            `yaml:"refresh_interval"` // seconds
	HTTPPort                     int            `yaml:"http_port"`
	LogLevel                     string         `yaml:"log_level"`
	APITimeout                   int            `yaml:"api_timeout"` // Azure API timeout in seconds
	EnableHighCardinalityMetrics *bool          `yaml:"enable_high_cardinality_metrics"` // Enable cloud_cost_daily_by_resource metric (default: true)
}

// Load loads configuration from a YAML file and applies environment variable overrides
func Load(path string) (*Config, error) {
	// Read YAML file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Override with environment variables
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("environment variable error: %w", err)
	}

	// Validate
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for configuration
func applyDefaults(cfg *Config) {
	if cfg.Currency == "" {
		cfg.Currency = DefaultCurrency
	}
	if cfg.DateRange.EndDateOffset == 0 {
		cfg.DateRange.EndDateOffset = DefaultEndDateOffset
	}
	if cfg.DateRange.DaysToQuery == 0 {
		cfg.DateRange.DaysToQuery = DefaultDaysToQuery
	}
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = DefaultRefreshInterval
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = DefaultHTTPPort
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = DefaultLogLevel
	}
	if cfg.APITimeout == 0 {
		cfg.APITimeout = DefaultAPITimeout
	}
	// Default: high-cardinality metrics enabled for backward compatibility
	// Users can disable them by setting enable_high_cardinality_metrics: false
	if cfg.EnableHighCardinalityMetrics == nil {
		enabled := true
		cfg.EnableHighCardinalityMetrics = &enabled
	}
}

// applyEnvOverrides applies environment variable overrides to configuration
func applyEnvOverrides(cfg *Config) error {
	// Override currency
	if val := os.Getenv("AZURE_COST_CURRENCY"); val != "" {
		cfg.Currency = val
	}

	// Override refresh interval
	if val := os.Getenv("AZURE_COST_REFRESH_INTERVAL"); val != "" {
		i, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid AZURE_COST_REFRESH_INTERVAL: must be an integer, got %q", val)
		}
		cfg.RefreshInterval = i
	}

	// Override HTTP port
	if val := os.Getenv("AZURE_COST_HTTP_PORT"); val != "" {
		i, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid AZURE_COST_HTTP_PORT: must be an integer, got %q", val)
		}
		cfg.HTTPPort = i
	}

	// Override log level
	if val := os.Getenv("AZURE_COST_LOG_LEVEL"); val != "" {
		cfg.LogLevel = val
	}

	// Override end date offset
	if val := os.Getenv("AZURE_COST_END_DATE_OFFSET"); val != "" {
		i, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid AZURE_COST_END_DATE_OFFSET: must be an integer, got %q", val)
		}
		cfg.DateRange.EndDateOffset = i
	}

	// Override days to query
	if val := os.Getenv("AZURE_COST_DAYS_TO_QUERY"); val != "" {
		i, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid AZURE_COST_DAYS_TO_QUERY: must be an integer, got %q", val)
		}
		cfg.DateRange.DaysToQuery = i
	}

	// Override subscriptions (comma-separated id:name pairs)
	// Example: AZURE_COST_SUBSCRIPTIONS="sub1:prod,sub2:dev"
	if val := os.Getenv("AZURE_COST_SUBSCRIPTIONS"); val != "" {
		subs := []Subscription{}
		for _, pair := range strings.Split(val, ",") {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				subs = append(subs, Subscription{
					ID:   strings.TrimSpace(parts[0]),
					Name: strings.TrimSpace(parts[1]),
				})
			} else if len(parts) == 1 {
				id := strings.TrimSpace(parts[0])
				subs = append(subs, Subscription{
					ID:   id,
					Name: id,
				})
			}
		}
		if len(subs) > 0 {
			cfg.Subscriptions = subs
		}
	}

	// Override high-cardinality metrics setting
	if val := os.Getenv("AZURE_COST_ENABLE_HIGH_CARDINALITY_METRICS"); val != "" {
		enabled := strings.ToLower(val) == "true" || val == "1"
		cfg.EnableHighCardinalityMetrics = &enabled
	}

	return nil
}

// validate validates the configuration
func validate(cfg *Config) error {
	if len(cfg.Subscriptions) == 0 {
		return fmt.Errorf("no subscriptions configured")
	}

	for i, sub := range cfg.Subscriptions {
		if sub.ID == "" {
			return fmt.Errorf("subscription at index %d has empty ID", i)
		}
		// Validate subscription name is not empty
		if sub.Name == "" {
			return fmt.Errorf("subscription at index %d has empty name", i)
		}
	}

	// Check for negative or zero refresh interval
	if cfg.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be positive, got %d", cfg.RefreshInterval)
	}

	if cfg.RefreshInterval < MinRefreshInterval {
		return fmt.Errorf("refresh_interval must be at least %d seconds", MinRefreshInterval)
	}

	// Validate date range
	if cfg.DateRange.DaysToQuery < MinDaysToQuery {
		return fmt.Errorf("days_to_query must be at least %d", MinDaysToQuery)
	}

	if cfg.DateRange.EndDateOffset < 0 {
		return fmt.Errorf("end_date_offset cannot be negative, got %d", cfg.DateRange.EndDateOffset)
	}

	// Ensure date range makes sense (can't query dates that haven't happened yet)
	if cfg.DateRange.EndDateOffset >= cfg.DateRange.DaysToQuery {
		return fmt.Errorf("end_date_offset (%d) must be less than days_to_query (%d)",
			cfg.DateRange.EndDateOffset, cfg.DateRange.DaysToQuery)
	}

	if cfg.HTTPPort < MinPort || cfg.HTTPPort > MaxPort {
		return fmt.Errorf("http_port must be between %d and %d", MinPort, MaxPort)
	}

	// Validate API timeout
	if cfg.APITimeout <= 0 {
		return fmt.Errorf("api_timeout must be positive, got %d", cfg.APITimeout)
	}

	if cfg.APITimeout > 300 {
		return fmt.Errorf("api_timeout should not exceed 300 seconds (5 minutes), got %d", cfg.APITimeout)
	}

	return nil
}
