package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig_Success(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
subscriptions:
  - id: "test-sub-1"
    name: "test-subscription"

currency: "€"

date_range:
  end_date_offset: 1
  days_to_query: 7

refresh_interval: 3600
http_port: 8080
log_level: "info"

group_by:
  enabled: true
  groups:
    - type: Dimension
      name: ServiceName
      label_name: ServiceName
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify parsed values
	if len(cfg.Subscriptions) != 1 {
		t.Errorf("Expected 1 subscription, got %d", len(cfg.Subscriptions))
	}
	if cfg.Subscriptions[0].ID != "test-sub-1" {
		t.Errorf("Subscription ID = %v, want test-sub-1", cfg.Subscriptions[0].ID)
	}
	if cfg.Currency != "€" {
		t.Errorf("Currency = %v, want €", cfg.Currency)
	}
	if cfg.RefreshInterval != 3600 {
		t.Errorf("RefreshInterval = %v, want 3600", cfg.RefreshInterval)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %v, want 8080", cfg.HTTPPort)
	}
}

func TestLoad_ApplyDefaults_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Minimal config with missing optional fields
	configContent := `
subscriptions:
  - id: "test-sub-1"
    name: "test"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify defaults
	tests := []struct {
		name string
		got  interface{}
		want interface{}
		desc string
	}{
		{"Currency", cfg.Currency, "€", "default currency"},
		{"EndDateOffset", cfg.DateRange.EndDateOffset, 1, "default end date offset"},
		{"DaysToQuery", cfg.DateRange.DaysToQuery, 7, "default days to query"},
		{"RefreshInterval", cfg.RefreshInterval, 3600, "default refresh interval"},
		{"HTTPPort", cfg.HTTPPort, 8080, "default HTTP port"},
		{"LogLevel", cfg.LogLevel, "info", "default log level"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.desc, tt.got, tt.want)
			}
		})
	}
}

func TestLoad_EnvOverrides_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
subscriptions:
  - id: "test-sub-1"
    name: "test"
currency: "€"
refresh_interval: 3600
http_port: 8080
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Set environment variables
	os.Setenv("AZURE_COST_CURRENCY", "$")
	os.Setenv("AZURE_COST_REFRESH_INTERVAL", "7200")
	os.Setenv("AZURE_COST_HTTP_PORT", "9090")
	os.Setenv("AZURE_COST_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("AZURE_COST_CURRENCY")
		os.Unsetenv("AZURE_COST_REFRESH_INTERVAL")
		os.Unsetenv("AZURE_COST_HTTP_PORT")
		os.Unsetenv("AZURE_COST_LOG_LEVEL")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify env overrides
	if cfg.Currency != "$" {
		t.Errorf("Currency = %v, want $ (env override)", cfg.Currency)
	}
	if cfg.RefreshInterval != 7200 {
		t.Errorf("RefreshInterval = %v, want 7200 (env override)", cfg.RefreshInterval)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %v, want 9090 (env override)", cfg.HTTPPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %v, want debug (env override)", cfg.LogLevel)
	}
}

func TestLoad_SubscriptionsEnvOverride_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
subscriptions:
  - id: "original-sub"
    name: "original"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Override subscriptions via env var
	os.Setenv("AZURE_COST_SUBSCRIPTIONS", "sub1:prod,sub2:dev,sub3")
	defer os.Unsetenv("AZURE_COST_SUBSCRIPTIONS")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify env override replaced original subscriptions
	if len(cfg.Subscriptions) != 3 {
		t.Fatalf("Expected 3 subscriptions from env, got %d", len(cfg.Subscriptions))
	}

	expected := []struct {
		id   string
		name string
	}{
		{"sub1", "prod"},
		{"sub2", "dev"},
		{"sub3", "sub3"}, // No name provided, should use ID
	}

	for i, exp := range expected {
		if cfg.Subscriptions[i].ID != exp.id {
			t.Errorf("Subscription[%d].ID = %v, want %v", i, cfg.Subscriptions[i].ID, exp.id)
		}
		if cfg.Subscriptions[i].Name != exp.name {
			t.Errorf("Subscription[%d].Name = %v, want %v", i, cfg.Subscriptions[i].Name, exp.name)
		}
	}
}

func TestValidate_EmptySubscriptions_Error(t *testing.T) {
	cfg := &Config{
		Subscriptions:   []Subscription{},
		RefreshInterval: 3600,
		HTTPPort:        8080,
		DateRange:       DateRange{DaysToQuery: 7},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("validate() error = nil, want error for empty subscriptions")
	}
}

func TestValidate_EmptySubscriptionID_Error(t *testing.T) {
	cfg := &Config{
		Subscriptions: []Subscription{
			{ID: "valid-sub", Name: "test"},
			{ID: "", Name: "invalid"},
		},
		RefreshInterval: 3600,
		HTTPPort:        8080,
		DateRange:       DateRange{DaysToQuery: 7},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("validate() error = nil, want error for empty subscription ID")
	}
}

func TestValidate_RefreshIntervalTooLow_Error(t *testing.T) {
	cfg := &Config{
		Subscriptions:   []Subscription{{ID: "test", Name: "test"}},
		RefreshInterval: 30, // Less than 60
		HTTPPort:        8080,
		DateRange:       DateRange{DaysToQuery: 7},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("validate() error = nil, want error for refresh_interval < 60")
	}
}

func TestValidate_InvalidHTTPPort_Error(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port too low", 0},
		{"port too high", 70000},
		{"negative port", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Subscriptions:   []Subscription{{ID: "test", Name: "test"}},
				RefreshInterval: 3600,
				HTTPPort:        tt.port,
				DateRange:       DateRange{DaysToQuery: 7},
			}

			err := validate(cfg)
			if err == nil {
				t.Errorf("validate() error = nil, want error for port %d", tt.port)
			}
		})
	}
}

func TestValidate_DaysToQueryTooLow_Error(t *testing.T) {
	cfg := &Config{
		Subscriptions:   []Subscription{{ID: "test", Name: "test"}},
		RefreshInterval: 3600,
		HTTPPort:        8080,
		DateRange:       DateRange{DaysToQuery: 0},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("validate() error = nil, want error for days_to_query < 1")
	}
}

func TestLoad_MissingFile_Error(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() error = nil, want error for missing file")
	}
}

func TestLoad_MalformedYAML_Error(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Invalid YAML - incorrect indentation and structure
	configContent := `
subscriptions:
  - id: "test"
    name: "test"
    invalid_nested:
- this: is
  : malformed
    yaml: [[[
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() error = nil, want error for malformed YAML")
	}
}
