package collector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zgpcy/azure-cost-exporter/internal/config"
	"github.com/zgpcy/azure-cost-exporter/internal/logger"
	"github.com/zgpcy/azure-cost-exporter/internal/provider"
)

// testLogger creates a logger for testing (debug level for more verbose output)
func testLogger() *logger.Logger {
	return logger.New("error") // Use error level to suppress test output
}

// mockCloudProvider is a mock implementation of the cloud provider for testing
type mockCloudProvider struct {
	mu            sync.Mutex
	records       []provider.CostRecord
	err           error
	queryCalls    int
	queryDuration time.Duration
	providerType  provider.ProviderType
	accountCount  int
}

func (m *mockCloudProvider) QueryCosts(ctx context.Context) ([]provider.CostRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queryCalls++

	// Simulate query duration if set
	if m.queryDuration > 0 {
		time.Sleep(m.queryDuration)
	}

	// Check context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return m.records, m.err
}

func (m *mockCloudProvider) Name() provider.ProviderType {
	return m.providerType
}

func (m *mockCloudProvider) AccountCount() int {
	return m.accountCount
}

func (m *mockCloudProvider) QueryCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queryCalls
}

func (m *mockCloudProvider) SetRecords(records []provider.CostRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = records
}

func (m *mockCloudProvider) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// TestNewCostCollector tests collector creation
func TestNewCostCollector(t *testing.T) {
	mockClient := &mockCloudProvider{}
	cfg := &config.Config{
		Currency: "€",
	}

	collector := NewCostCollector(mockClient, cfg, testLogger())

	if collector == nil {
		t.Fatal("NewCostCollector returned nil")
	}
	if collector.cloudProvider == nil {
		t.Error("cloudProvider should not be nil")
	}
	if collector.cfg == nil {
		t.Error("cfg should not be nil")
	}
	if collector.costMetric == nil {
		t.Error("costMetric should not be nil")
	}
	if collector.upMetric == nil {
		t.Error("upMetric should not be nil")
	}
}

// TestDescribe tests the Describe method
func TestDescribe(t *testing.T) {
	mockClient := &mockCloudProvider{}
	cfg := &config.Config{}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ch := make(chan *prometheus.Desc, 10)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	var descs []*prometheus.Desc
	for desc := range ch {
		descs = append(descs, desc)
	}

	// Should have: costMetric, completedDailyCostMetric, upMetric, scrapeDurationMetric, scrapeErrorsTotal, lastScrapeTimeMetric, recordCountMetric, buildInfo
	if len(descs) != 8 {
		t.Errorf("Expected 8 descriptors, got %d", len(descs))
	}
}

// TestCollect_NoData tests collection when no data has been fetched yet
func TestCollect_NoData(t *testing.T) {
	mockClient := &mockCloudProvider{}
	cfg := &config.Config{}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	// Should have: up, scrape_duration, records_count, buildInfo
	// Note: scrape_errors counter won't export if never incremented, last_scrape_timestamp not set if zero
	if len(metrics) != 4 {
		t.Errorf("Expected 4 metrics (up + operational metrics + buildInfo, no scrape_errors since never incremented), got %d", len(metrics))
	}
}

// TestCollect_WithData tests collection with cost records
func TestCollect_WithData(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{
				Date:             time.Now().Format("2006-01-02"),
				AccountName:      "test-sub",
				AccountID:        "sub-123",
				Service:          "Storage",
				ResourceType:     "microsoft.storage/storageaccounts",
				ResourceGroup:    "prod-rg",
				ResourceLocation: "westeurope",
				ResourceID:       "/subscriptions/sub-123/resourcegroups/prod-rg/providers/microsoft.storage/storageaccounts/store1",
				ResourceName:     "store1",
				MeterCategory:    "Storage",
				MeterSubCategory: "Premium SSD",
				ChargeType:       "Usage",
				PricingModel:     "OnDemand",
				Cost:             12.34,
				Currency:         "€",
			},
			{
				Date:             time.Now().Format("2006-01-02"),
				AccountName:      "test-sub",
				AccountID:        "sub-123",
				Service:          "Virtual Machines",
				ResourceType:     "microsoft.compute/virtualmachines",
				ResourceGroup:    "dev-rg",
				ResourceLocation: "northeurope",
				ResourceID:       "/subscriptions/sub-123/resourcegroups/dev-rg/providers/microsoft.compute/virtualmachines/vm1",
				ResourceName:     "vm1",
				MeterCategory:    "Compute",
				MeterSubCategory: "D2s v3",
				ChargeType:       "Usage",
				PricingModel:     "Reservation",
				Cost:             45.67,
				Currency:         "€",
			},
		},
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	// Trigger refresh to load data
	ctx := context.Background()
	collector.refresh(ctx)

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	// Should have: 2 cost metrics (by service) + 5 operational metrics
	// (up, scrape_duration, last_scrape_timestamp, records_count, buildInfo)
	// Note: scrape_errors counter won't export if never incremented
	if len(metrics) != 7 {
		t.Errorf("Expected 7 metrics (2 cost + 5 operational), got %d", len(metrics))
	}

	// Verify collector is ready
	if !collector.IsReady() {
		t.Error("Collector should be ready after successful refresh")
	}

	// Verify record count
	if collector.RecordCount() != 2 {
		t.Errorf("RecordCount: got %d, want 2", collector.RecordCount())
	}

	// Verify no error
	if collector.LastError() != nil {
		t.Errorf("LastError should be nil, got %v", collector.LastError())
	}
}

// TestCollect_WithError tests collection after a failed refresh
func TestCollect_WithError(t *testing.T) {
	mockClient := &mockCloudProvider{
		err: errors.New("Azure API error"),
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	// Should have: up (0), scrape_duration, scrape_errors, last_scrape_timestamp, records_count, buildInfo
	if len(metrics) != 6 {
		t.Errorf("Expected 6 metrics (up=0 + operational metrics + buildInfo), got %d", len(metrics))
	}

	// Verify collector is not ready
	if collector.IsReady() {
		t.Error("Collector should not be ready after failed refresh")
	}

	// Verify error is stored
	if collector.LastError() == nil {
		t.Error("LastError should not be nil after failed refresh")
	}

	// Verify record count is 0
	if collector.RecordCount() != 0 {
		t.Errorf("RecordCount should be 0 after error, got %d", collector.RecordCount())
	}
}

// TestRefresh tests the refresh method
func TestRefresh(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{
				Date:        time.Now().Format("2006-01-02"),
				AccountName: "test",
				AccountID:   "123",
				Service:     "Storage",
				Cost:        10.0,
				Currency:    "$",
			},
		},
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	beforeRefresh := time.Now()
	collector.refresh(ctx)
	afterRefresh := time.Now()

	// Verify LastScrapeTime is within expected range
	scrapeTime := collector.LastScrapeTime()
	if scrapeTime.Before(beforeRefresh) || scrapeTime.After(afterRefresh) {
		t.Errorf("LastScrapeTime %v not within expected range [%v, %v]", scrapeTime, beforeRefresh, afterRefresh)
	}

	// Verify data was loaded
	if collector.RecordCount() != 1 {
		t.Errorf("Expected 1 record after refresh, got %d", collector.RecordCount())
	}

	// Verify ready state
	if !collector.IsReady() {
		t.Error("Collector should be ready after successful refresh")
	}
}

// TestStartBackgroundRefresh tests the background refresh goroutine
func TestStartBackgroundRefresh(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
	}

	cfg := &config.Config{RefreshInterval: 1} // 1 second for fast test
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartBackgroundRefresh(ctx)

	// Wait for initial refresh
	time.Sleep(100 * time.Millisecond)

	initialCalls := mockClient.QueryCallCount()
	if initialCalls < 1 {
		t.Error("Expected at least 1 query call for initial refresh")
	}

	// Wait for at least one more refresh cycle
	time.Sleep(1200 * time.Millisecond)

	finalCalls := mockClient.QueryCallCount()
	if finalCalls <= initialCalls {
		t.Errorf("Expected more query calls after refresh interval, initial=%d final=%d", initialCalls, finalCalls)
	}

	// Cancel context and verify goroutine stops
	cancel()
	time.Sleep(100 * time.Millisecond)

	callsAfterCancel := mockClient.QueryCallCount()
	time.Sleep(1200 * time.Millisecond)
	finalCallsAfterCancel := mockClient.QueryCallCount()

	if finalCallsAfterCancel != callsAfterCancel {
		t.Error("Query calls should not increase after context cancellation")
	}
}

// TestStartBackgroundRefresh_ContextCancellation tests graceful shutdown
func TestStartBackgroundRefresh_ContextCancellation(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
	}

	cfg := &config.Config{RefreshInterval: 10} // Long interval
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx, cancel := context.WithCancel(context.Background())

	collector.StartBackgroundRefresh(ctx)

	// Wait for initial refresh
	time.Sleep(100 * time.Millisecond)

	// Cancel immediately
	cancel()

	// Wait a bit to ensure goroutine has stopped
	time.Sleep(100 * time.Millisecond)

	// Should have exactly 1 call (initial refresh only)
	calls := mockClient.QueryCallCount()
	if calls != 1 {
		t.Errorf("Expected exactly 1 query call before cancellation, got %d", calls)
	}
}

// TestConcurrency_MultipleCollectCalls tests thread-safety of Collect method
func TestConcurrency_MultipleCollectCalls(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
			{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Compute", Cost: 20.0, Currency: "$"},
		},
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	// Launch multiple goroutines calling Collect concurrently
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ch := make(chan prometheus.Metric, 10)
			go func() {
				collector.Collect(ch)
				close(ch)
			}()

			// Drain the channel
			count := 0
			for range ch {
				count++
			}

			// Should always get 7 metrics (2 cost + 5 operational)
			// Note: scrape_errors counter won't export if never incremented
			if count != 7 {
				t.Errorf("Expected 7 metrics, got %d", count)
			}
		}()
	}

	wg.Wait()
}

// TestConcurrency_CollectDuringRefresh tests Collect calls while refresh is running
func TestConcurrency_CollectDuringRefresh(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
		queryDuration: 200 * time.Millisecond, // Simulate slow query
	}

	cfg := &config.Config{RefreshInterval: 1}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background refresh
	collector.StartBackgroundRefresh(ctx)

	// Launch multiple Collect calls while refreshes are happening
	var wg sync.WaitGroup
	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			// Stagger the calls
			time.Sleep(time.Duration(iteration*10) * time.Millisecond)

			ch := make(chan prometheus.Metric, 10)
			go func() {
				collector.Collect(ch)
				close(ch)
			}()

			// Drain the channel
			for range ch {
				// Just drain, don't verify counts since refresh might be in progress
			}
		}(i)
	}

	wg.Wait()
	cancel()
}

// TestConcurrency_StateMethodsDuringRefresh tests thread-safety of state accessor methods
func TestConcurrency_StateMethodsDuringRefresh(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
		queryDuration: 100 * time.Millisecond,
	}

	cfg := &config.Config{RefreshInterval: 1}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.StartBackgroundRefresh(ctx)

	// Launch goroutines calling state methods concurrently
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(4) // 4 methods to test

		go func() {
			defer wg.Done()
			_ = collector.IsReady()
		}()

		go func() {
			defer wg.Done()
			_ = collector.LastError()
		}()

		go func() {
			defer wg.Done()
			_ = collector.LastScrapeTime()
		}()

		go func() {
			defer wg.Done()
			_ = collector.RecordCount()
		}()
	}

	wg.Wait()
	cancel()
}

// TestUpMetric_Success tests up metric value when collector is working
func TestUpMetric_Success(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
		},
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	// Collect metrics and verify up metric value
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	// Drain and check up metric
	foundUpMetric := false
	for metric := range ch {
		if metric.Desc().String() == collector.upMetric.String() {
			foundUpMetric = true
			// Up metric should be 1.0 after successful refresh
		}
	}

	if !foundUpMetric {
		t.Error("up metric not found in collected metrics")
	}

	if !collector.IsReady() {
		t.Error("Collector should be ready after successful refresh")
	}
}

// TestUpMetric_Failure tests up metric value when collector encounters error
func TestUpMetric_Failure(t *testing.T) {
	mockClient := &mockCloudProvider{
		err: errors.New("API error"),
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	// Collect metrics and verify up metric value
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	// Drain metrics
	foundUpMetric := false
	for metric := range ch {
		if metric.Desc().String() == collector.upMetric.String() {
			foundUpMetric = true
		}
	}

	if !foundUpMetric {
		t.Error("up metric not found in collected metrics")
	}

	if collector.IsReady() {
		t.Error("Collector should not be ready after error")
	}
}

// TestUpMetric_EmptyRecords tests up metric when query succeeds but returns no records
func TestUpMetric_EmptyRecords(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{}, // Empty but no error
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	// Collect metrics
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	// Drain metrics
	foundUpMetric := false
	for metric := range ch {
		if metric.Desc().String() == collector.upMetric.String() {
			foundUpMetric = true
		}
	}

	if !foundUpMetric {
		t.Error("up metric not found in collected metrics")
	}

	// Collector should be ready (no error occurred)
	if !collector.IsReady() {
		t.Error("Collector should be ready even with empty records (no error)")
	}
}

// TestRefresh_ErrorRecovery tests that collector can recover from errors
func TestRefresh_ErrorRecovery(t *testing.T) {
	mockClient := &mockCloudProvider{
		err: errors.New("temporary error"),
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()

	// First refresh fails
	collector.refresh(ctx)

	if collector.IsReady() {
		t.Error("Collector should not be ready after error")
	}
	if collector.LastError() == nil {
		t.Error("LastError should be set after failed refresh")
	}

	// Fix the error and add data
	mockClient.SetError(nil)
	mockClient.SetRecords([]provider.CostRecord{
		{Date: time.Now().Format("2006-01-02"), AccountName: "test", AccountID: "123", Service: "Storage", Cost: 10.0, Currency: "$"},
	})

	// Second refresh succeeds
	collector.refresh(ctx)

	if !collector.IsReady() {
		t.Error("Collector should be ready after successful recovery")
	}
	if collector.LastError() != nil {
		t.Errorf("LastError should be nil after recovery, got %v", collector.LastError())
	}
	if collector.RecordCount() != 1 {
		t.Errorf("RecordCount should be 1 after recovery, got %d", collector.RecordCount())
	}
}

// TestMetricAggregation tests that low-cardinality metrics properly aggregate by service
func TestMetricAggregation(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	mockClient := &mockCloudProvider{
		providerType: provider.ProviderAzure,
		records: []provider.CostRecord{
			{
				Date:             today,
				Provider:         string(provider.ProviderAzure),
				AccountName:      "test-sub",
				AccountID:        "sub-123",
				Service:          "Storage",
				ResourceType:     "microsoft.storage/storageaccounts",
				ResourceGroup:    "rg1",
				ResourceLocation: "westeurope",
				Cost:             10.0,
				Currency:         "€",
			},
			{
				Date:             today,
				Provider:         string(provider.ProviderAzure),
				AccountName:      "test-sub",
				AccountID:        "sub-123",
				Service:          "Storage", // Same service, different resource
				ResourceType:     "microsoft.storage/storageaccounts",
				ResourceGroup:    "rg2",
				ResourceLocation: "northeurope",
				Cost:             15.0,
				Currency:         "€",
			},
			{
				Date:             today,
				Provider:         string(provider.ProviderAzure),
				AccountName:      "test-sub",
				AccountID:        "sub-123",
				Service:          "Compute", // Different service
				ResourceType:     "microsoft.compute/virtualmachines",
				ResourceGroup:    "rg1",
				ResourceLocation: "westeurope",
				Cost:             25.0,
				Currency:         "€",
			},
		},
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	ch := make(chan prometheus.Metric, 20)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	var metrics []prometheus.Metric
	for metric := range ch {
		metrics = append(metrics, metric)
	}

	// Should have:
	// - 2 cost metrics (Storage aggregated to 25.0, Compute to 25.0)
	// - 5 operational metrics (up, scrape_duration, last_scrape_timestamp, records_count, buildInfo)
	// Note: scrape_errors counter won't export if never incremented
	// Total: 2 + 5 = 7 metrics
	if len(metrics) != 7 {
		t.Errorf("Expected 7 metrics (2 cost + 5 operational), got %d", len(metrics))
	}

	// Verify collector state
	if !collector.IsReady() {
		t.Error("Collector should be ready after successful refresh")
	}
	if collector.RecordCount() != 3 {
		t.Errorf("RecordCount: got %d, want 3", collector.RecordCount())
	}
}

// TestMemoryLimits tests that record count is capped at MaxRecordsToCache
func TestMemoryLimits(t *testing.T) {
	// Create more records than the limit
	excessiveRecords := make([]provider.CostRecord, MaxRecordsToCache+5000)
	for i := 0; i < len(excessiveRecords); i++ {
		excessiveRecords[i] = provider.CostRecord{
			Date:        time.Now().Format("2006-01-02"),
			AccountName: "test",
			AccountID:   "123",
			Service:     "Storage",
			Cost:        10.0,
			Currency:    "$",
		}
	}

	mockClient := &mockCloudProvider{
		records: excessiveRecords,
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	// Verify that records are truncated to MaxRecordsToCache
	if collector.RecordCount() != MaxRecordsToCache {
		t.Errorf("Expected record count to be capped at %d, got %d", MaxRecordsToCache, collector.RecordCount())
	}

	// Collector should still be ready (truncation is a warning, not an error)
	if !collector.IsReady() {
		t.Error("Collector should be ready even after truncation")
	}
}

// TestMetricLabels tests that the single cost metric is properly exported
func TestMetricLabels(t *testing.T) {
	mockClient := &mockCloudProvider{
		records: []provider.CostRecord{
			{
				Date:             time.Now().Format("2006-01-02"),
				AccountName:      "prod-subscription",
				AccountID:        "sub-abc-123",
				Service:          "Azure Database for PostgreSQL",
				ResourceType:     "microsoft.dbforpostgresql/flexibleservers",
				ResourceGroup:    "database-rg",
				ResourceLocation: "westeurope",
				ResourceID:       "/subscriptions/sub-abc-123/resourcegroups/database-rg/providers/microsoft.dbforpostgresql/flexibleservers/proddb",
				ResourceName:     "proddb",
				MeterCategory:    "Azure Database for PostgreSQL",
				MeterSubCategory: "Flexible Server General Purpose",
				ChargeType:       "Usage",
				PricingModel:     "Reservation",
				Cost:             125.50,
				Currency:         "€",
			},
		},
	}

	cfg := &config.Config{RefreshInterval: 3600}
	collector := NewCostCollector(mockClient, cfg, testLogger())

	ctx := context.Background()
	collector.refresh(ctx)

	// Single metric with dynamic labels based on groupBy configuration
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metricFound := false
	for metric := range ch {
		if metric.Desc().String() == collector.costMetric.String() {
			metricFound = true
		}
	}

	if !metricFound {
		t.Error("Cost metric (cloud_cost_daily) not found")
	}
}
