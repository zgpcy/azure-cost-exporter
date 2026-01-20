package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"
	"github.com/cenkalti/backoff/v4"
	"github.com/zgpcy/azure-cost-exporter/internal/clock"
	"github.com/zgpcy/azure-cost-exporter/internal/config"
	"github.com/zgpcy/azure-cost-exporter/internal/logger"
	"github.com/zgpcy/azure-cost-exporter/internal/provider"
)

// Azure API retry constants
const (
	// MaxRetryElapsedTime is the maximum time to spend retrying a failed API call
	MaxRetryElapsedTime = 2 * time.Minute

	// InitialRetryInterval is the initial backoff interval for retries
	InitialRetryInterval = 1 * time.Second

	// MaxRetryInterval is the maximum backoff interval between retries
	MaxRetryInterval = 30 * time.Second
)

// Client wraps the Azure Cost Management client and implements provider.CloudProvider
type Client struct {
	client *armcostmanagement.QueryClient
	cfg    *config.Config
	logger *logger.Logger
	clock  clock.Clock // Time provider for testing
}

// Verify that Client implements provider.CloudProvider
var _ provider.CloudProvider = (*Client)(nil)

// NewClient creates a new Azure Cost Management client
func NewClient(cfg *config.Config, log *logger.Logger) (*Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := armcostmanagement.NewQueryClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost management client: %w", err)
	}

	return &Client{
		client: client,
		cfg:    cfg,
		logger: log,
		clock:  clock.RealClock{}, // Use real system time by default
	}, nil
}

// Name returns the provider type
func (c *Client) Name() provider.ProviderType {
	return provider.ProviderAzure
}

// AccountCount returns the number of Azure subscriptions being monitored
func (c *Client) AccountCount() int {
	return len(c.cfg.Subscriptions)
}

// QueryCosts retrieves cost data for all configured subscriptions
// Returns partial data if some subscriptions fail (best-effort approach)
func (c *Client) QueryCosts(ctx context.Context) ([]provider.CostRecord, error) {
	var (
		allRecords []provider.CostRecord
		errors     []error
	)

	// Pre-allocate with estimated capacity
	estimatedRecordsPerSub := 1000
	allRecords = make([]provider.CostRecord, 0, len(c.cfg.Subscriptions)*estimatedRecordsPerSub)

	for _, sub := range c.cfg.Subscriptions {
		records, err := c.queryCostsForSubscription(ctx, sub)
		if err != nil {
			// Log the error but continue with other subscriptions
			c.logger.Warn("Failed to query subscription, continuing with others",
				"subscription_name", sub.Name,
				"subscription_id", sub.ID,
				"error", err)
			errors = append(errors, fmt.Errorf("subscription %s: %w", sub.Name, err))
			continue
		}
		allRecords = append(allRecords, records...)
	}

	// Only return error if ALL subscriptions failed
	if len(errors) > 0 && len(allRecords) == 0 {
		return nil, fmt.Errorf("all %d subscriptions failed (check Azure credentials and permissions): %v",
			len(c.cfg.Subscriptions), errors)
	}

	// Log warning if some subscriptions failed but we have partial data
	if len(errors) > 0 {
		c.logger.Warn("Some subscriptions failed, returning partial data",
			"failed_count", len(errors),
			"total_subscriptions", len(c.cfg.Subscriptions),
			"records_returned", len(allRecords))
	}

	// Return partial data with success (Prometheus best practice: partial data > no data)
	return allRecords, nil
}

// queryCostsForSubscription queries costs for a single subscription with retry logic
func (c *Client) queryCostsForSubscription(ctx context.Context, sub config.Subscription) ([]provider.CostRecord, error) {
	var result []provider.CostRecord

	// Configure exponential backoff
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = InitialRetryInterval
	bo.MaxInterval = MaxRetryInterval
	bo.MaxElapsedTime = MaxRetryElapsedTime

	operation := func() error {
		records, err := c.queryCostsForSubscriptionInternal(ctx, sub)
		if err != nil {
			// Log retry attempt with context
			c.logger.Debug("Azure API call failed, will retry",
				"subscription_name", sub.Name,
				"subscription_id", sub.ID,
				"error", err)
			return err
		}
		result = records
		return nil
	}

	// Retry with exponential backoff
	if err := backoff.Retry(operation, backoff.WithContext(bo, ctx)); err != nil {
		return nil, fmt.Errorf("subscription %s (ID: %s) failed after retries: %w", sub.Name, sub.ID, err)
	}

	return result, nil
}

// queryCostsForSubscriptionInternal performs the actual API call without retry logic
func (c *Client) queryCostsForSubscriptionInternal(ctx context.Context, sub config.Subscription) ([]provider.CostRecord, error) {
	// Create context with timeout for API call (from config)
	apiTimeout := time.Duration(c.cfg.APITimeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	// Calculate date range
	endDateOffset := 0
	if c.cfg.DateRange.EndDateOffset != nil {
		endDateOffset = *c.cfg.DateRange.EndDateOffset
	}
	endDate := c.clock.Now().AddDate(0, 0, -endDateOffset)
	startDate := endDate.AddDate(0, 0, -(c.cfg.DateRange.DaysToQuery - 1))

	// Truncate to beginning of day in UTC
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)

	c.logger.Debug("Querying Azure Cost Management API",
		"subscription", sub.Name,
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
		"current_time", c.clock.Now().Format("2006-01-02 15:04:05 MST"))

	// Build grouping
	var grouping []*armcostmanagement.QueryGrouping
	if c.cfg.GroupBy.Enabled {
		for _, g := range c.cfg.GroupBy.Groups {
			groupType := armcostmanagement.QueryColumnType(g.Type)
			grouping = append(grouping, &armcostmanagement.QueryGrouping{
				Type: &groupType,
				Name: &g.Name,
			})
		}
	}

	// Build query definition
	scope := fmt.Sprintf("/subscriptions/%s", sub.ID)
	queryType := armcostmanagement.ExportTypeActualCost
	timeframe := armcostmanagement.TimeframeTypeCustom
	granularity := armcostmanagement.GranularityTypeDaily

	aggregation := map[string]*armcostmanagement.QueryAggregation{
		"totalCost": {
			Name:     stringPtr("Cost"),
			Function: functionPtr(armcostmanagement.FunctionTypeSum),
		},
	}

	queryDef := armcostmanagement.QueryDefinition{
		Type:      &queryType,
		Timeframe: &timeframe,
		TimePeriod: &armcostmanagement.QueryTimePeriod{
			From: &startDate,
			To:   &endDate,
		},
		Dataset: &armcostmanagement.QueryDataset{
			Granularity: &granularity,
			Aggregation: aggregation,
			Grouping:    grouping,
		},
	}

	// Execute query
	resp, err := c.client.Usage(ctx, scope, queryDef, nil)
	if err != nil {
		return nil, fmt.Errorf("cost query failed for date range %s to %s: %w",
			startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), err)
	}

	// Parse response
	return c.parseResponse(resp.QueryResult, sub), nil
}

// buildColumnMap creates a map of column names to their indices
func buildColumnMap(columns []*armcostmanagement.QueryColumn) map[string]int {
	columnMap := make(map[string]int)
	for i, col := range columns {
		if col.Name != nil {
			columnMap[*col.Name] = i
		}
	}
	return columnMap
}

// getStringFromRow extracts a string value from a row by column name
func getStringFromRow(row []interface{}, columnMap map[string]int, columnName string) string {
	if idx, ok := columnMap[columnName]; ok && len(row) > idx {
		value := fmt.Sprintf("%v", row[idx])
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

// parseCost extracts and converts cost value to float64
func parseCost(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0.0
	}
}

// formatDateValue converts various date types to string
func formatDateValue(value interface{}) string {
	switch v := value.(type) {
	case int, int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractDigits extracts only digit characters from a string
func extractDigits(s string) string {
	var digits strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
		}
	}
	return digits.String()
}

// formatYYYYMMDD formats date digits as YYYY-MM-DD
func formatYYYYMMDD(dateDigits string) string {
	if len(dateDigits) >= 8 {
		return fmt.Sprintf("%s-%s-%s",
			dateDigits[0:4],
			dateDigits[4:6],
			dateDigits[6:8])
	}
	return dateDigits
}

// parseDate extracts and formats date from various input types
func parseDate(value interface{}) string {
	dateVal := formatDateValue(value)
	dateDigits := extractDigits(dateVal)
	return formatYYYYMMDD(dateDigits)
}

// extractResourceInfo extracts resource ID and name from a row
func extractResourceInfo(row []interface{}, columnMap map[string]int) (string, string) {
	resourceId := ""
	resourceName := ""

	if idx, ok := columnMap["ResourceId"]; ok && len(row) > idx {
		resourceId = fmt.Sprintf("%v", row[idx])
		// Extract resource name from ResourceId (last segment after /)
		// Skip if empty or "<nil>"
		if resourceId != "" && resourceId != "<nil>" {
			parts := strings.Split(resourceId, "/")
			if len(parts) > 0 {
				resourceName = parts[len(parts)-1]
			}
		}
	}

	return resourceId, resourceName
}

// extractService extracts service name with fallback to MeterCategory
func extractService(row []interface{}, columnMap map[string]int) string {
	// Try ServiceName first
	if service := getStringFromRow(row, columnMap, "ServiceName"); service != "" {
		return service
	}

	// Fallback to MeterCategory
	if meterCat := getStringFromRow(row, columnMap, "MeterCategory"); meterCat != "" {
		return meterCat
	}

	// Final fallback
	return "Unknown"
}

// extractResourceGroup extracts resource group with fallback to ResourceGroupName
func extractResourceGroup(row []interface{}, columnMap map[string]int) string {
	if rg := getStringFromRow(row, columnMap, "ResourceGroup"); rg != "" {
		return rg
	}
	return getStringFromRow(row, columnMap, "ResourceGroupName")
}

// parseRow parses a single row from the Azure API response
func (c *Client) parseRow(row []interface{}, columnMap map[string]int, costIdx, dateIdx int, sub config.Subscription) provider.CostRecord {
	cost := parseCost(row[costIdx])
	date := parseDate(row[dateIdx])

	service := extractService(row, columnMap)
	resourceId, resourceName := extractResourceInfo(row, columnMap)

	return provider.CostRecord{
		Date:             date,
		Provider:         string(provider.ProviderAzure),
		AccountID:        sub.ID,
		AccountName:      sub.Name,
		Service:          service,
		ResourceType:     getStringFromRow(row, columnMap, "ResourceType"),
		ResourceGroup:    extractResourceGroup(row, columnMap),
		ResourceLocation: getStringFromRow(row, columnMap, "ResourceLocation"),
		ResourceID:       resourceId,
		ResourceName:     resourceName,
		MeterCategory:    getStringFromRow(row, columnMap, "MeterCategory"),
		MeterSubCategory: getStringFromRow(row, columnMap, "MeterSubCategory"),
		ChargeType:       getStringFromRow(row, columnMap, "ChargeType"),
		PricingModel:     getStringFromRow(row, columnMap, "PricingModel"),
		Cost:             cost,
		Currency:         c.cfg.Currency,
	}
}

// parseResponse converts Azure API response to CostRecords
func (c *Client) parseResponse(result armcostmanagement.QueryResult, sub config.Subscription) []provider.CostRecord {
	var records []provider.CostRecord

	if result.Properties == nil || result.Properties.Rows == nil {
		return records
	}

	// Build column index map
	columnMap := buildColumnMap(result.Properties.Columns)


	// Verify required columns exist
	costIdx, hasCost := columnMap["Cost"]
	dateIdx, hasDate := columnMap["UsageDate"]

	if !hasCost || !hasDate {
		return records
	}

	// Parse each row
	for _, row := range result.Properties.Rows {
		if len(row) <= costIdx || len(row) <= dateIdx {
			continue
		}

		record := c.parseRow(row, columnMap, costIdx, dateIdx, sub)
		records = append(records, record)
	}

	return records
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func functionPtr(f armcostmanagement.FunctionType) *armcostmanagement.FunctionType {
	return &f
}
