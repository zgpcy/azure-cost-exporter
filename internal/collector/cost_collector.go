package collector

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zgpcy/azure-cost-exporter/internal/clock"
	"github.com/zgpcy/azure-cost-exporter/internal/config"
	"github.com/zgpcy/azure-cost-exporter/internal/logger"
	"github.com/zgpcy/azure-cost-exporter/internal/provider"
	"github.com/zgpcy/azure-cost-exporter/internal/version"
)

// MaxRecordsToCache limits memory usage by capping the number of cached cost records
// At ~200 bytes per record, 100K records = ~20MB
const MaxRecordsToCache = 100000

// buildMetricLabels builds the label names for the cost metric
// based on the groupBy configuration
func buildMetricLabels(cfg *config.Config) []string {
	// Base labels always present
	labels := []string{"provider", "account_name", "account_id", "service"}

	// Add dynamic labels from groupBy configuration
	if cfg.GroupBy.Enabled {
		for _, group := range cfg.GroupBy.Groups {
			labels = append(labels, group.LabelName)
		}
	}

	// Trailing labels always present
	labels = append(labels, "date", "currency")

	return labels
}

// extractLabelValues extracts label values from a CostRecord based on label names
func extractLabelValues(record provider.CostRecord, labelNames []string) []string {
	values := make([]string, len(labelNames))

	for i, labelName := range labelNames {
		switch labelName {
		case "provider":
			values[i] = record.Provider
		case "account_name":
			values[i] = record.AccountName
		case "account_id":
			values[i] = record.AccountID
		case "service":
			values[i] = record.Service
		case "resource_type":
			values[i] = record.ResourceType
		case "resource_group":
			values[i] = record.ResourceGroup
		case "resource_location":
			values[i] = record.ResourceLocation
		case "resource_id":
			values[i] = record.ResourceID
		case "resource_name":
			values[i] = record.ResourceName
		case "meter_category":
			values[i] = record.MeterCategory
		case "meter_subcategory":
			values[i] = record.MeterSubCategory
		case "charge_type":
			values[i] = record.ChargeType
		case "pricing_model":
			values[i] = record.PricingModel
		case "date":
			values[i] = record.Date
		case "currency":
			values[i] = record.Currency
		default:
			values[i] = ""
		}
	}

	return values
}

// CostCollector implements prometheus.Collector for cloud cost metrics
type CostCollector struct {
	cloudProvider provider.CloudProvider
	cfg           *config.Config
	logger        *logger.Logger
	clock         clock.Clock // Time provider for testing

	// Metrics
	costMetric           *prometheus.Desc
	costMetricLabelNames []string                       // Dynamic label names from groupBy config
	upMetric             *prometheus.Desc
	scrapeDurationMetric *prometheus.Desc
	scrapeErrorsTotal    *prometheus.CounterVec // Proper counter metric
	lastScrapeTimeMetric *prometheus.Desc
	recordCountMetric    *prometheus.Desc
	buildInfo            *prometheus.GaugeVec // Build version information

	// State
	mu                 sync.RWMutex
	lastRecords        []provider.CostRecord
	lastError          error
	lastScrape         time.Time
	lastScrapeDuration time.Duration
	refreshStarted     atomic.Bool // Prevent multiple refresh goroutines
	isReady            bool
}

// NewCostCollector creates a new CostCollector
func NewCostCollector(cloudProvider provider.CloudProvider, cfg *config.Config, log *logger.Logger) *CostCollector {
	// Create proper counter metric for scrape errors
	scrapeErrorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloud_cost_exporter_scrape_errors_total",
			Help: "Total number of cloud cost data scrape errors since startup",
		},
		[]string{"provider"},
	)

	// Create build info metric
	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloud_cost_exporter_build_info",
			Help: "Build version information",
		},
		[]string{"version", "git_commit", "build_date", "go_version"},
	)

	// Set build info to 1 with version labels
	versionInfo := version.Info()
	buildInfo.With(prometheus.Labels{
		"version":    versionInfo["version"],
		"git_commit": versionInfo["git_commit"],
		"build_date": versionInfo["build_date"],
		"go_version": versionInfo["go_version"],
	}).Set(1)

	// Build dynamic label names from groupBy configuration
	metricLabels := buildMetricLabels(cfg)

	return &CostCollector{
		cloudProvider: cloudProvider,
		cfg:           cfg,
		logger:        log,
		clock:         clock.RealClock{}, // Use real system time by default
		// Cost metric with dynamic labels based on groupBy configuration
		costMetric: prometheus.NewDesc(
			"cloud_cost_daily",
			"Daily cloud cost with grouping dimensions. Labels are dynamically generated from groupBy configuration.",
			metricLabels,
			nil,
		),
		costMetricLabelNames: metricLabels,
		upMetric: prometheus.NewDesc(
			"up",
			"Was the last cloud cost query successful (1 = success, 0 = failure)",
			[]string{"provider"},
			nil,
		),
		scrapeDurationMetric: prometheus.NewDesc(
			"cloud_cost_exporter_scrape_duration_seconds",
			"Duration of the last cloud cost data scrape in seconds",
			[]string{"provider"},
			nil,
		),
		scrapeErrorsTotal: scrapeErrorsTotal,
		lastScrapeTimeMetric: prometheus.NewDesc(
			"cloud_cost_exporter_last_scrape_timestamp_seconds",
			"Unix timestamp of the last successful scrape",
			[]string{"provider"},
			nil,
		),
		recordCountMetric: prometheus.NewDesc(
			"cloud_cost_exporter_records_count",
			"Number of cost records currently cached",
			[]string{"provider"},
			nil,
		),
		buildInfo: buildInfo,
	}
}

// Describe implements prometheus.Collector
func (c *CostCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.costMetric
	ch <- c.upMetric
	ch <- c.scrapeDurationMetric
	c.scrapeErrorsTotal.Describe(ch) // Describe the counter
	ch <- c.lastScrapeTimeMetric
	ch <- c.recordCountMetric
	c.buildInfo.Describe(ch) // Describe build info
}

// Collect implements prometheus.Collector
func (c *CostCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	providerName := string(c.cloudProvider.Name())

	// Aggregate costs by label values
	// Key is a string representation of all label values joined together
	type labelKey string
	costs := make(map[labelKey]struct {
		labelValues []string
		cost        float64
	})

	// Aggregate costs for all records
	for _, record := range c.lastRecords {
		// Extract label values dynamically based on configured label names
		labelValues := extractLabelValues(record, c.costMetricLabelNames)

		// Create unique key from label values
		key := labelKey(strings.Join(labelValues, "|"))

		// Aggregate costs with identical label values
		existing := costs[key]
		existing.labelValues = labelValues
		existing.cost += record.Cost
		costs[key] = existing
	}

	// Export aggregated cost metrics
	for _, data := range costs {
		ch <- prometheus.MustNewConstMetric(
			c.costMetric,
			prometheus.GaugeValue,
			data.cost,
			data.labelValues...,
		)
	}

	// Send up metric
	upValue := 0.0
	if c.lastError == nil && len(c.lastRecords) > 0 {
		upValue = 1.0
	}
	ch <- prometheus.MustNewConstMetric(
		c.upMetric,
		prometheus.GaugeValue,
		upValue,
		providerName,
	)

	// Send scrape duration metric
	ch <- prometheus.MustNewConstMetric(
		c.scrapeDurationMetric,
		prometheus.GaugeValue,
		c.lastScrapeDuration.Seconds(),
		providerName,
	)

	// Collect scrape errors counter (proper counter that survives across scrapes)
	c.scrapeErrorsTotal.Collect(ch)

	// Send last scrape time metric
	if !c.lastScrape.IsZero() {
		ch <- prometheus.MustNewConstMetric(
			c.lastScrapeTimeMetric,
			prometheus.GaugeValue,
			float64(c.lastScrape.Unix()),
			providerName,
		)
	}

	// Send record count metric
	ch <- prometheus.MustNewConstMetric(
		c.recordCountMetric,
		prometheus.GaugeValue,
		float64(len(c.lastRecords)),
		providerName,
	)

	// Collect build info metric
	c.buildInfo.Collect(ch)
}

// StartBackgroundRefresh starts a goroutine that periodically refreshes cost data
// Uses atomic flag to prevent multiple refresh goroutines
func (c *CostCollector) StartBackgroundRefresh(ctx context.Context) {
	// Prevent multiple refresh goroutines
	if !c.refreshStarted.CompareAndSwap(false, true) {
		c.logger.Warn("Background refresh already started, skipping")
		return
	}

	// Initial fetch
	c.refresh(ctx)

	// Background refresh loop
	ticker := time.NewTicker(time.Duration(c.cfg.RefreshInterval) * time.Second)
	go func() {
		defer ticker.Stop()
		defer c.refreshStarted.Store(false) // Reset on exit
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Stopping background refresh")
				return
			case <-ticker.C:
				c.refresh(ctx)
			}
		}
	}()
}

// refresh queries the cloud provider and updates the cached data
func (c *CostCollector) refresh(ctx context.Context) {
	providerName := c.cloudProvider.Name()
	c.logger.Info("Refreshing cost data", "provider", providerName)
	start := time.Now()

	records, err := c.cloudProvider.QueryCosts(ctx)
	duration := time.Since(start)

	// Enforce memory limits
	if len(records) > MaxRecordsToCache {
		c.logger.Warn("Received records exceeding limit, truncating to prevent memory issues",
			"received_count", len(records),
			"limit", MaxRecordsToCache)
		records = records[:MaxRecordsToCache]
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastScrape = c.clock.Now()
	c.lastScrapeDuration = duration
	c.lastError = err
	c.lastRecords = records

	if err != nil {
		c.scrapeErrorsTotal.With(prometheus.Labels{"provider": string(providerName)}).Inc()
		c.logger.Error("Failed to refresh cost data", "provider", providerName, "error", err)
		c.isReady = false
		return
	}

	c.isReady = true
	c.logger.Info("Successfully refreshed cost records",
		"provider", providerName,
		"record_count", len(records),
		"duration_seconds", duration.Seconds())
}

// IsReady returns true if the collector has successfully fetched data at least once
func (c *CostCollector) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isReady
}

// LastError returns the last error encountered during refresh
func (c *CostCollector) LastError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

// LastScrapeTime returns the time of the last scrape attempt
func (c *CostCollector) LastScrapeTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastScrape
}

// RecordCount returns the number of cost records currently cached
func (c *CostCollector) RecordCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.lastRecords)
}
