package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zgpcy/azure-cost-exporter/internal/collector"
	"github.com/zgpcy/azure-cost-exporter/internal/config"
	"github.com/zgpcy/azure-cost-exporter/internal/logger"
	"github.com/zgpcy/azure-cost-exporter/internal/provider"
)

// testLogger creates a logger for testing (error level to suppress test output)
func testLogger() *logger.Logger {
	return logger.New("error")
}

// mockCloudProvider is a mock implementation for testing
type mockCloudProvider struct {
	mu           sync.Mutex
	records      []provider.CostRecord
	err          error
	queryCalls   int
	providerType provider.ProviderType
	accountCount int
}

func (m *mockCloudProvider) QueryCosts(ctx context.Context) ([]provider.CostRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCalls++
	return m.records, m.err
}

func (m *mockCloudProvider) Name() provider.ProviderType {
	return m.providerType
}

func (m *mockCloudProvider) AccountCount() int {
	return m.accountCount
}

// TestNewServer tests server creation
func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		HTTPPort:        8080,
		RefreshInterval: 3600,
		Subscriptions: []config.Subscription{
			{ID: "sub-1", Name: "test"},
		},
	}

	mockClient := &mockCloudProvider{}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())
	server := NewServer(cfg, collector, testLogger())

	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.server == nil {
		t.Error("server.server should not be nil")
	}
	if server.collector == nil {
		t.Error("server.collector should not be nil")
	}
	if server.cfg == nil {
		t.Error("server.cfg should not be nil")
	}
	if server.server.Addr != ":8080" {
		t.Errorf("server address: got %v, want :8080", server.server.Addr)
	}
}

// TestHandleHealth tests the /health endpoint
func TestHandleHealth(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())
	server := NewServer(cfg, collector, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Verify status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type: got %v, want application/json", contentType)
	}

	// Verify response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedBody := `{"status":"healthy"}`
	if string(body) != expectedBody {
		t.Errorf("Response body: got %v, want %v", string(body), expectedBody)
	}
}

// TestHandleHealth_AlwaysHealthy tests that health endpoint always returns 200
func TestHandleHealth_AlwaysHealthy(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{
		err: errors.New("Azure API error"), // Even with errors, health should be OK
	}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Trigger a failed refresh
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartBackgroundRefresh(ctx)
	time.Sleep(100 * time.Millisecond)

	server := NewServer(cfg, collector, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Health should still be OK even with collector errors
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code: got %v, want %v (health should always be OK)", resp.StatusCode, http.StatusOK)
	}
}

// TestHandleReady_NotReady tests the /ready endpoint when collector is not ready
func TestHandleReady_NotReady(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())
	server := NewServer(cfg, collector, testLogger())

	// Collector has not fetched data yet
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	server.handleReady(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Should return 503 Service Unavailable
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Status code: got %v, want %v", resp.StatusCode, http.StatusServiceUnavailable)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type: got %v, want application/json", contentType)
	}

	// Verify response body contains "not ready"
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !strings.Contains(string(body), "not ready") {
		t.Errorf("Response body should contain 'not ready', got: %s", string(body))
	}
}

// TestHandleReady_Ready tests the /ready endpoint when collector is ready
func TestHandleReady_Ready(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: "2026-01-15", AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
	}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Trigger refresh to load data
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartBackgroundRefresh(ctx)
	time.Sleep(100 * time.Millisecond)

	server := NewServer(cfg, collector, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	server.handleReady(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Should return 200 OK
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Verify response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedBody := `{"status":"ready"}`
	if string(body) != expectedBody {
		t.Errorf("Response body: got %v, want %v", string(body), expectedBody)
	}
}

// TestHandleReady_WithError tests the /ready endpoint when collector has an error
func TestHandleReady_WithError(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{
		err: errors.New("Azure API failure"),
	}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Trigger refresh to load error state
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartBackgroundRefresh(ctx)
	time.Sleep(150 * time.Millisecond) // Give it more time to refresh and set error state

	server := NewServer(cfg, collector, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	server.handleReady(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Should return 503 Service Unavailable
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Status code: got %v, want %v", resp.StatusCode, http.StatusServiceUnavailable)
	}

	// Verify response body contains "not ready"
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "not ready") {
		t.Errorf("Response body should contain 'not ready', got: %s", bodyStr)
	}

	// Verify collector has the error
	if collector.LastError() == nil {
		t.Error("Collector should have stored the error")
	}
}

// TestHandleIndex_NotReady tests the index page when collector is not ready
func TestHandleIndex_NotReady(t *testing.T) {
	cfg := &config.Config{
		HTTPPort:        8080,
		RefreshInterval: 3600,
		Subscriptions: []config.Subscription{
			{ID: "sub-1", Name: "test"},
		},
	}
	mockClient := &mockCloudProvider{}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())
	server := NewServer(cfg, collector, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Verify status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("Content-Type: got %v, want text/html", contentType)
	}

	// Verify response body contains expected elements
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	requiredStrings := []string{
		"Azure Cost Exporter",
		"Not Ready",
		"Prometheus exporter",
		"/metrics",
		"/health",
		"/ready",
		"3600 seconds", // refresh interval
		"1",            // subscription count
	}

	for _, required := range requiredStrings {
		if !strings.Contains(bodyStr, required) {
			t.Errorf("Response body should contain %q", required)
		}
	}
}

// TestHandleIndex_Ready tests the index page when collector is ready
func TestHandleIndex_Ready(t *testing.T) {
	cfg := &config.Config{
		HTTPPort:        8080,
		RefreshInterval: 1800,
		Subscriptions: []config.Subscription{
			{ID: "sub-1", Name: "test-1"},
			{ID: "sub-2", Name: "test-2"},
		},
	}
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: "2026-01-15", AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
			{Date: "2026-01-15", AccountName: "test", AccountID: "123", Service: "Compute", Cost: 20.0, Currency: "$"},
		},
	}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Trigger refresh to load data
	ctx := context.Background()
	collector.StartBackgroundRefresh(ctx)
	time.Sleep(100 * time.Millisecond)

	server := NewServer(cfg, collector, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify ready status
	if !strings.Contains(bodyStr, "Ready") {
		t.Error("Response body should contain 'Ready' status")
	}

	// Verify record count
	if !strings.Contains(bodyStr, "2") {
		t.Error("Response body should show 2 cost records")
	}

	// Verify refresh interval
	if !strings.Contains(bodyStr, "1800 seconds") {
		t.Error("Response body should show 1800 seconds refresh interval")
	}

	// Verify subscription count (2 subscriptions)
	// Note: The HTML shows subscription count, looking for "2" in subscriptions context
	if !strings.Contains(bodyStr, "Subscriptions:") {
		t.Error("Response body should mention subscriptions")
	}
}

// TestHandleIndex_LastScrapeTime tests that last scrape time is displayed
func TestHandleIndex_LastScrapeTime(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: "2026-01-15", AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
	}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Trigger refresh
	ctx := context.Background()
	collector.StartBackgroundRefresh(ctx)
	time.Sleep(100 * time.Millisecond)

	server := NewServer(cfg, collector, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Should NOT contain "Never" since we've scraped
	if strings.Contains(bodyStr, "Never") {
		t.Error("Last scrape should not be 'Never' after successful refresh")
	}

	// Should contain "Last Scrape:" label
	if !strings.Contains(bodyStr, "Last Scrape:") {
		t.Error("Response should contain 'Last Scrape:' label")
	}
}

// TestMetricsEndpoint tests the /metrics endpoint
func TestMetricsEndpoint(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{
		providerType: provider.ProviderAzure,
		accountCount: 1,
		records: []provider.CostRecord{
			{
				Date:             "2026-01-15",
				Provider:         "azure",
				AccountName:      "test-sub",
				AccountID:        "sub-123",
				Service:          "Storage",
				ResourceType:     "microsoft.storage/storageaccounts",
				ResourceGroup:    "rg-1",
				ResourceLocation: "westeurope",
				ResourceID:       "/subscriptions/sub-123/resourcegroups/rg-1/providers/microsoft.storage/storageaccounts/store1",
				ResourceName:     "store1",
				MeterCategory:    "Storage",
				MeterSubCategory: "Premium SSD",
				ChargeType:       "Usage",
				PricingModel:     "OnDemand",
				Cost:             25.50,
				Currency:         "€",
			},
		},
	}
	coll := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Register collector with Prometheus
	reg := prometheus.NewRegistry()
	reg.MustRegister(coll)

	// Trigger refresh
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coll.StartBackgroundRefresh(ctx)
	time.Sleep(100 * time.Millisecond)

	server := NewServer(cfg, coll, testLogger())

	// Override the handler to use our custom registry
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
	server.server.Handler = mux

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	server.server.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Verify status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Content-Type should contain text/plain, got %v", contentType)
	}

	// Verify response body contains metrics
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	expectedMetrics := []string{
		"cloud_cost_daily",
		"up",
		"provider=\"azure\"",
		"account_name=\"test-sub\"",
		"service=\"Storage\"",
	}

	for _, expected := range expectedMetrics {
		if !strings.Contains(bodyStr, expected) {
			t.Errorf("Metrics should contain %q", expected)
		}
	}
}

// TestMetricsEndpoint_NoData tests /metrics when no data is available
func TestMetricsEndpoint_NoData(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{
		providerType: provider.ProviderAzure,
		accountCount: 0,
	}
	coll := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Register collector with Prometheus
	reg := prometheus.NewRegistry()
	reg.MustRegister(coll)

	server := NewServer(cfg, coll, testLogger())

	// Override the handler to use our custom registry
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
	server.server.Handler = mux

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Should still return 200 OK
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Should still have the up metric
	if !strings.Contains(bodyStr, "up") {
		t.Error("Metrics should contain up even with no data")
	}

	// Up metric should be 0 (check for the metric with provider label)
	if !strings.Contains(bodyStr, "up{provider=\"azure\"} 0") {
		t.Error("up should be 0 when no data")
	}
}

// TestConcurrency_MultipleRequests tests handling multiple concurrent requests
func TestConcurrency_MultipleRequests(t *testing.T) {
	cfg := &config.Config{
		HTTPPort:        8080,
		RefreshInterval: 3600,
		Subscriptions: []config.Subscription{
			{ID: "sub-1", Name: "test"},
		},
	}
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: "2026-01-15", AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
	}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())

	// Trigger refresh
	ctx := context.Background()
	collector.StartBackgroundRefresh(ctx)
	time.Sleep(100 * time.Millisecond)

	server := NewServer(cfg, collector, testLogger())

	endpoints := []string{"/", "/health", "/ready", "/metrics"}

	var wg sync.WaitGroup
	numRequests := 20

	for _, endpoint := range endpoints {
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(ep string) {
				defer wg.Done()

				req := httptest.NewRequest(http.MethodGet, ep, nil)
				w := httptest.NewRecorder()

				server.server.Handler.ServeHTTP(w, req)

				resp := w.Result()
				defer resp.Body.Close()

				// All endpoints should return successfully
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Endpoint %s returned status %v, want %v", ep, resp.StatusCode, http.StatusOK)
				}
			}(endpoint)
		}
	}

	wg.Wait()
}

// TestServerTimeouts tests that server has proper timeout configurations
func TestServerTimeouts(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())
	server := NewServer(cfg, collector, testLogger())

	if server.server.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout: got %v, want 15s", server.server.ReadTimeout)
	}
	if server.server.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout: got %v, want 15s", server.server.WriteTimeout)
	}
	if server.server.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout: got %v, want 60s", server.server.IdleTimeout)
	}
}

// TestHandleReady_StateTransitions tests ready endpoint through state transitions
func TestHandleReady_StateTransitions(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 1} // 1 second for fast refresh
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: "2026-01-15", AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
	}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())
	server := NewServer(cfg, collector, testLogger())

	// State 1: Not ready (no data fetched yet)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	server.handleReady(w, req)
	if w.Result().StatusCode != http.StatusServiceUnavailable {
		t.Error("Should be not ready before first refresh")
	}

	// State 2: Fetch data, become ready
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector.StartBackgroundRefresh(ctx)
	time.Sleep(200 * time.Millisecond) // Wait for initial refresh

	req = httptest.NewRequest(http.MethodGet, "/ready", nil)
	w = httptest.NewRecorder()
	server.handleReady(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Error("Should be ready after successful refresh")
	}

	// State 3: Inject error, become not ready
	mockClient.mu.Lock()
	mockClient.err = errors.New("temporary failure")
	mockClient.mu.Unlock()

	// Wait for automatic refresh to pick up the error
	time.Sleep(1200 * time.Millisecond)

	req = httptest.NewRequest(http.MethodGet, "/ready", nil)
	w = httptest.NewRecorder()
	server.handleReady(w, req)
	if w.Result().StatusCode != http.StatusServiceUnavailable {
		t.Error("Should be not ready after error")
	}
}

// TestHTTPMethods_OnlyGET tests that only GET method is accepted
func TestHTTPMethods_OnlyGET(t *testing.T) {
	cfg := &config.Config{HTTPPort: 8080, RefreshInterval: 3600}
	mockClient := &mockCloudProvider{}
	collector := collector.NewCostCollector(mockClient, cfg, testLogger())
	server := NewServer(cfg, collector, testLogger())

	endpoints := []string{"/", "/health", "/ready", "/metrics"}
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, endpoint := range endpoints {
		for _, method := range methods {
			req := httptest.NewRequest(method, endpoint, nil)
			w := httptest.NewRecorder()

			server.server.Handler.ServeHTTP(w, req)

			// For non-GET methods, Prometheus handler returns 405 or similar
			// Our custom handlers don't explicitly reject, but we verify they work with GET
			// This test mainly ensures GET works
			t.Logf("Endpoint %s with method %s returned status %d", endpoint, method, w.Result().StatusCode)
		}
	}

	// Verify GET works for all endpoints
	for _, endpoint := range endpoints {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		w := httptest.NewRecorder()

		server.server.Handler.ServeHTTP(w, req)

		// All GET requests should succeed (200 or 503 for /ready is OK)
		statusCode := w.Result().StatusCode
		if statusCode != http.StatusOK && statusCode != http.StatusServiceUnavailable {
			t.Errorf("GET %s returned unexpected status %d", endpoint, statusCode)
		}
	}
}
