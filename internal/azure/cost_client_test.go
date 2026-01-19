package azure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"
	"github.com/zgpcy/azure-cost-exporter/internal/config"
)

// TestParseResponse_FullResponse tests parsing a complete Azure API response with all dimensions
func TestParseResponse_FullResponse(t *testing.T) {
	client, sub := setupTestClient(t)

	// Load full mock response
	result := loadMockResponse(t, "mock_response_full.json")

	// Parse response
	records := client.parseResponse(result, sub)

	// Verify we got 2 records
	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	// Verify first record (Storage)
	r1 := records[0]
	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Cost", r1.Cost, 12.34},
		{"Date", r1.Date, "2026-01-15"},
		{"Service", r1.Service, "Storage"},
		{"ResourceGroup", r1.ResourceGroup, "production-rg"},
		{"ResourceID", r1.ResourceID, "/subscriptions/test-sub-1/resourcegroups/production-rg/providers/microsoft.storage/storageaccounts/prodstore123"},
		{"ResourceName", r1.ResourceName, "prodstore123"},
		{"MeterCategory", r1.MeterCategory, "Storage"},
		{"MeterSubCategory", r1.MeterSubCategory, "Premium SSD Managed Disks"},
		{"ResourceType", r1.ResourceType, "microsoft.storage/storageaccounts"},
		{"ResourceLocation", r1.ResourceLocation, "EU North"},
		{"ChargeType", r1.ChargeType, "Usage"},
		{"PricingModel", r1.PricingModel, "OnDemand"},
		{"AccountName", r1.AccountName, "test-subscription"},
		{"AccountID", r1.AccountID, "test-sub-1"},
		{"Currency", r1.Currency, "€"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	// Verify second record (PostgreSQL with Reservation pricing)
	r2 := records[1]
	if r2.Cost != 45.67 {
		t.Errorf("Record 2 Cost: got %v, want 45.67", r2.Cost)
	}
	if r2.Service != "Azure Database for PostgreSQL" {
		t.Errorf("Record 2 Service: got %v, want 'Azure Database for PostgreSQL'", r2.Service)
	}
	if r2.ResourceName != "test-db" {
		t.Errorf("Record 2 ResourceName: got %v, want 'test-db'", r2.ResourceName)
	}
	if r2.PricingModel != "Reservation" {
		t.Errorf("Record 2 PricingModel: got %v, want 'Reservation'", r2.PricingModel)
	}
}

// TestParseResponse_MinimalResponse tests parsing with minimal dimensions
func TestParseResponse_MinimalResponse(t *testing.T) {
	client, sub := setupTestClient(t)

	result := loadMockResponse(t, "mock_response_minimal.json")
	records := client.parseResponse(result, sub)

	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	r1 := records[0]

	// Verify basic fields present
	if r1.Cost != 10.50 {
		t.Errorf("Cost: got %v, want 10.50", r1.Cost)
	}
	if r1.Date != "2026-01-15" {
		t.Errorf("Date: got %v, want 2026-01-15", r1.Date)
	}
	if r1.Service != "Storage" {
		t.Errorf("Service: got %v, want Storage", r1.Service)
	}

	// Verify optional fields are empty when not present in response
	if r1.ResourceGroup != "" {
		t.Errorf("ResourceGroup should be empty, got %v", r1.ResourceGroup)
	}
	if r1.ResourceID != "" {
		t.Errorf("ResourceID should be empty, got %v", r1.ResourceID)
	}
	if r1.ResourceName != "" {
		t.Errorf("ResourceName should be empty, got %v", r1.ResourceName)
	}
}

// TestParseResponse_EmptyResponse tests handling of empty results
func TestParseResponse_EmptyResponse(t *testing.T) {
	client, sub := setupTestClient(t)

	result := loadMockResponse(t, "mock_response_empty.json")
	records := client.parseResponse(result, sub)

	if len(records) != 0 {
		t.Errorf("Expected 0 records for empty response, got %d", len(records))
	}
}

// TestParseResponse_NilProperties tests handling of nil properties
func TestParseResponse_NilProperties(t *testing.T) {
	client, sub := setupTestClient(t)

	result := armcostmanagement.QueryResult{
		Properties: nil,
	}
	records := client.parseResponse(result, sub)

	if len(records) != 0 {
		t.Errorf("Expected 0 records for nil properties, got %d", len(records))
	}
}

// TestParseResponse_NilRows tests handling of nil rows
func TestParseResponse_NilRows(t *testing.T) {
	client, sub := setupTestClient(t)

	result := armcostmanagement.QueryResult{
		Properties: &armcostmanagement.QueryProperties{
			Columns: []*armcostmanagement.QueryColumn{},
			Rows:    nil,
		},
	}
	records := client.parseResponse(result, sub)

	if len(records) != 0 {
		t.Errorf("Expected 0 records for nil rows, got %d", len(records))
	}
}

// TestParseResponse_MissingRequiredColumns tests handling when Cost or UsageDate columns are missing
func TestParseResponse_MissingRequiredColumns(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name    string
		columns []*armcostmanagement.QueryColumn
	}{
		{
			name: "missing Cost column",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("ServiceName"), Type: stringPtr("String")},
			},
		},
		{
			name: "missing UsageDate column",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("ServiceName"), Type: stringPtr("String")},
			},
		},
		{
			name:    "no columns",
			columns: []*armcostmanagement.QueryColumn{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: tt.columns,
					Rows: [][]interface{}{
						{10.50, 20260115, "Storage"},
					},
				},
			}
			records := client.parseResponse(result, sub)

			if len(records) != 0 {
				t.Errorf("Expected 0 records when required columns missing, got %d", len(records))
			}
		})
	}
}

// TestDateParsing tests various date format conversions
func TestDateParsing(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name         string
		dateValue    interface{}
		expectedDate string
	}{
		{"int date", 20260115, "2026-01-15"},
		{"int64 date", int64(20260116), "2026-01-16"},
		{"float64 date", float64(20260117), "2026-01-17"},
		{"string date digits", "20260118", "2026-01-18"},
		{"string date formatted", "2026-01-19", "2026-01-19"},
		{"string with spaces", "  20260120  ", "2026-01-20"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: []*armcostmanagement.QueryColumn{
						{Name: stringPtr("Cost"), Type: stringPtr("Number")},
						{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
					},
					Rows: [][]interface{}{
						{10.0, tt.dateValue},
					},
				},
			}

			records := client.parseResponse(result, sub)

			if len(records) != 1 {
				t.Fatalf("Expected 1 record, got %d", len(records))
			}

			if records[0].Date != tt.expectedDate {
				t.Errorf("Date parsing: got %v, want %v", records[0].Date, tt.expectedDate)
			}
		})
	}
}

// TestDateParsing_MalformedDates tests handling of malformed date values
func TestDateParsing_MalformedDates(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name      string
		dateValue interface{}
	}{
		{"short digit string", "202601"},
		{"single digit", "1"},
		{"empty string", ""},
		{"non-numeric string", "invalid"},
		{"negative number", -20260115},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: []*armcostmanagement.QueryColumn{
						{Name: stringPtr("Cost"), Type: stringPtr("Number")},
						{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
					},
					Rows: [][]interface{}{
						{10.0, tt.dateValue},
					},
				},
			}

			records := client.parseResponse(result, sub)

			if len(records) != 1 {
				t.Fatalf("Expected 1 record even with malformed date, got %d", len(records))
			}

			// Just verify it doesn't crash - date will be malformed but record should exist
			if records[0].Cost != 10.0 {
				t.Errorf("Cost should still be parsed correctly: got %v, want 10.0", records[0].Cost)
			}
		})
	}
}

// TestCostParsing tests various cost value types
func TestCostParsing(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name         string
		costValue    interface{}
		expectedCost float64
	}{
		{"float64 cost", 12.34, 12.34},
		{"int cost", 10, 10.0},
		{"int64 cost", int64(25), 25.0},
		{"zero cost", 0, 0.0},
		{"large cost", 999999.99, 999999.99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: []*armcostmanagement.QueryColumn{
						{Name: stringPtr("Cost"), Type: stringPtr("Number")},
						{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
					},
					Rows: [][]interface{}{
						{tt.costValue, 20260115},
					},
				},
			}

			records := client.parseResponse(result, sub)

			if len(records) != 1 {
				t.Fatalf("Expected 1 record, got %d", len(records))
			}

			if records[0].Cost != tt.expectedCost {
				t.Errorf("Cost parsing: got %v, want %v", records[0].Cost, tt.expectedCost)
			}
		})
	}
}

// TestCostParsing_InvalidTypes tests handling of invalid cost types (should default to 0)
func TestCostParsing_InvalidTypes(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name      string
		costValue interface{}
	}{
		{"string cost", "invalid"},
		{"nil cost", nil},
		{"boolean cost", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: []*armcostmanagement.QueryColumn{
						{Name: stringPtr("Cost"), Type: stringPtr("Number")},
						{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
					},
					Rows: [][]interface{}{
						{tt.costValue, 20260115},
					},
				},
			}

			records := client.parseResponse(result, sub)

			if len(records) != 1 {
				t.Fatalf("Expected 1 record, got %d", len(records))
			}

			if records[0].Cost != 0.0 {
				t.Errorf("Invalid cost type should default to 0.0, got %v", records[0].Cost)
			}
		})
	}
}

// TestResourceNameExtraction tests extraction of resource name from ResourceID
func TestResourceNameExtraction(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name         string
		resourceId   string
		expectedName string
	}{
		{
			"storage account",
			"/subscriptions/test-sub/resourcegroups/prod-rg/providers/microsoft.storage/storageaccounts/prodstore123",
			"prodstore123",
		},
		{
			"virtual machine",
			"/subscriptions/test-sub/resourcegroups/dev-rg/providers/microsoft.compute/virtualmachines/dev-vm-01",
			"dev-vm-01",
		},
		{
			"postgresql database",
			"/subscriptions/test-sub/resourcegroups/db-rg/providers/microsoft.dbforpostgresql/flexibleservers/test-db",
			"test-db",
		},
		{
			"single segment",
			"just-a-name",
			"just-a-name",
		},
		{
			"trailing slash",
			"/subscriptions/test-sub/resourcegroups/rg/providers/microsoft.test/resources/myresource/",
			"",
		},
		{
			"empty string",
			"",
			"",
		},
		{
			"nil marker",
			"<nil>",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: []*armcostmanagement.QueryColumn{
						{Name: stringPtr("Cost"), Type: stringPtr("Number")},
						{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
						{Name: stringPtr("ResourceId"), Type: stringPtr("String")},
					},
					Rows: [][]interface{}{
						{10.0, 20260115, tt.resourceId},
					},
				},
			}

			records := client.parseResponse(result, sub)

			if len(records) != 1 {
				t.Fatalf("Expected 1 record, got %d", len(records))
			}

			if records[0].ResourceName != tt.expectedName {
				t.Errorf("ResourceName extraction: got %q, want %q", records[0].ResourceName, tt.expectedName)
			}
			if records[0].ResourceID != tt.resourceId {
				t.Errorf("ResourceID should be preserved: got %q, want %q", records[0].ResourceID, tt.resourceId)
			}
		})
	}
}

// TestServiceNameFallback tests the service name fallback logic
func TestServiceNameFallback(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name            string
		columns         []*armcostmanagement.QueryColumn
		row             []interface{}
		expectedService string
	}{
		{
			name: "ServiceName present",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("ServiceName"), Type: stringPtr("String")},
			},
			row:             []interface{}{10.0, 20260115, "Virtual Machines"},
			expectedService: "Virtual Machines",
		},
		{
			name: "ServiceName missing, use MeterCategory",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("MeterCategory"), Type: stringPtr("String")},
			},
			row:             []interface{}{10.0, 20260115, "Storage"},
			expectedService: "Storage",
		},
		{
			name: "ServiceName empty, use MeterCategory",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("ServiceName"), Type: stringPtr("String")},
				{Name: stringPtr("MeterCategory"), Type: stringPtr("String")},
			},
			row:             []interface{}{10.0, 20260115, "", "Networking"},
			expectedService: "Networking",
		},
		{
			name: "Both missing, use Unknown",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
			},
			row:             []interface{}{10.0, 20260115},
			expectedService: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: tt.columns,
					Rows:    [][]interface{}{tt.row},
				},
			}

			records := client.parseResponse(result, sub)

			if len(records) != 1 {
				t.Fatalf("Expected 1 record, got %d", len(records))
			}

			if records[0].Service != tt.expectedService {
				t.Errorf("Service name fallback: got %q, want %q", records[0].Service, tt.expectedService)
			}
		})
	}
}

// TestResourceGroupFallback tests ResourceGroup vs ResourceGroupName column handling
func TestResourceGroupFallback(t *testing.T) {
	client, sub := setupTestClient(t)

	tests := []struct {
		name          string
		columns       []*armcostmanagement.QueryColumn
		row           []interface{}
		expectedGroup string
	}{
		{
			name: "ResourceGroup present",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("ResourceGroup"), Type: stringPtr("String")},
			},
			row:           []interface{}{10.0, 20260115, "production-rg"},
			expectedGroup: "production-rg",
		},
		{
			name: "ResourceGroupName as fallback",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("ResourceGroupName"), Type: stringPtr("String")},
			},
			row:           []interface{}{10.0, 20260115, "dev-rg"},
			expectedGroup: "dev-rg",
		},
		{
			name: "ResourceGroup takes precedence over ResourceGroupName",
			columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("ResourceGroup"), Type: stringPtr("String")},
				{Name: stringPtr("ResourceGroupName"), Type: stringPtr("String")},
			},
			row:           []interface{}{10.0, 20260115, "primary-rg", "fallback-rg"},
			expectedGroup: "primary-rg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := armcostmanagement.QueryResult{
				Properties: &armcostmanagement.QueryProperties{
					Columns: tt.columns,
					Rows:    [][]interface{}{tt.row},
				},
			}

			records := client.parseResponse(result, sub)

			if len(records) != 1 {
				t.Fatalf("Expected 1 record, got %d", len(records))
			}

			if records[0].ResourceGroup != tt.expectedGroup {
				t.Errorf("ResourceGroup fallback: got %q, want %q", records[0].ResourceGroup, tt.expectedGroup)
			}
		})
	}
}

// TestParseResponse_AllDimensionsPresent tests that all dimension fields are correctly extracted
func TestParseResponse_AllDimensionsPresent(t *testing.T) {
	client, sub := setupTestClient(t)

	columns := []*armcostmanagement.QueryColumn{
		{Name: stringPtr("Cost"), Type: stringPtr("Number")},
		{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
		{Name: stringPtr("ServiceName"), Type: stringPtr("String")},
		{Name: stringPtr("ResourceType"), Type: stringPtr("String")},
		{Name: stringPtr("ResourceGroup"), Type: stringPtr("String")},
		{Name: stringPtr("ResourceLocation"), Type: stringPtr("String")},
		{Name: stringPtr("ResourceId"), Type: stringPtr("String")},
		{Name: stringPtr("MeterCategory"), Type: stringPtr("String")},
		{Name: stringPtr("MeterSubCategory"), Type: stringPtr("String")},
		{Name: stringPtr("ChargeType"), Type: stringPtr("String")},
		{Name: stringPtr("PricingModel"), Type: stringPtr("String")},
	}

	row := []interface{}{
		100.50,                // Cost
		20260115,              // UsageDate
		"Test Service",        // ServiceName
		"microsoft.test/type", // ResourceType
		"test-rg",             // ResourceGroup
		"westeurope",          // ResourceLocation
		"/subscriptions/sub/resourcegroups/rg/providers/microsoft.test/resources/testresource", // ResourceID
		"Test Category",    // MeterCategory
		"Test SubCategory", // MeterSubCategory
		"Usage",            // ChargeType
		"Reservation",      // PricingModel
	}

	result := armcostmanagement.QueryResult{
		Properties: &armcostmanagement.QueryProperties{
			Columns: columns,
			Rows:    [][]interface{}{row},
		},
	}

	records := client.parseResponse(result, sub)

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	r := records[0]

	expectations := map[string]interface{}{
		"Cost":             100.50,
		"Date":             "2026-01-15",
		"Service":          "Test Service",
		"ResourceType":     "microsoft.test/type",
		"ResourceGroup":    "test-rg",
		"ResourceLocation": "westeurope",
		"ResourceID":       "/subscriptions/sub/resourcegroups/rg/providers/microsoft.test/resources/testresource",
		"ResourceName":     "testresource",
		"MeterCategory":    "Test Category",
		"MeterSubCategory": "Test SubCategory",
		"ChargeType":       "Usage",
		"PricingModel":     "Reservation",
		"Currency":         "€",
		"AccountName":      "test-subscription",
		"AccountID":        "test-sub-1",
	}

	// Use reflection-free approach for checking all fields
	if r.Cost != expectations["Cost"] {
		t.Errorf("Cost: got %v, want %v", r.Cost, expectations["Cost"])
	}
	if r.Date != expectations["Date"] {
		t.Errorf("Date: got %v, want %v", r.Date, expectations["Date"])
	}
	if r.Service != expectations["Service"] {
		t.Errorf("Service: got %v, want %v", r.Service, expectations["Service"])
	}
	if r.ResourceType != expectations["ResourceType"] {
		t.Errorf("ResourceType: got %v, want %v", r.ResourceType, expectations["ResourceType"])
	}
	if r.ResourceGroup != expectations["ResourceGroup"] {
		t.Errorf("ResourceGroup: got %v, want %v", r.ResourceGroup, expectations["ResourceGroup"])
	}
	if r.ResourceLocation != expectations["ResourceLocation"] {
		t.Errorf("ResourceLocation: got %v, want %v", r.ResourceLocation, expectations["ResourceLocation"])
	}
	if r.ResourceID != expectations["ResourceID"] {
		t.Errorf("ResourceID: got %v, want %v", r.ResourceID, expectations["ResourceID"])
	}
	if r.ResourceName != expectations["ResourceName"] {
		t.Errorf("ResourceName: got %v, want %v", r.ResourceName, expectations["ResourceName"])
	}
	if r.MeterCategory != expectations["MeterCategory"] {
		t.Errorf("MeterCategory: got %v, want %v", r.MeterCategory, expectations["MeterCategory"])
	}
	if r.MeterSubCategory != expectations["MeterSubCategory"] {
		t.Errorf("MeterSubCategory: got %v, want %v", r.MeterSubCategory, expectations["MeterSubCategory"])
	}
	if r.ChargeType != expectations["ChargeType"] {
		t.Errorf("ChargeType: got %v, want %v", r.ChargeType, expectations["ChargeType"])
	}
	if r.PricingModel != expectations["PricingModel"] {
		t.Errorf("PricingModel: got %v, want %v", r.PricingModel, expectations["PricingModel"])
	}
	if r.Currency != expectations["Currency"] {
		t.Errorf("Currency: got %v, want %v", r.Currency, expectations["Currency"])
	}
	if r.AccountName != expectations["AccountName"] {
		t.Errorf("AccountName: got %v, want %v", r.AccountName, expectations["AccountName"])
	}
	if r.AccountID != expectations["AccountID"] {
		t.Errorf("AccountID: got %v, want %v", r.AccountID, expectations["AccountID"])
	}
}

// TestParseResponse_ShortRow tests handling of rows with fewer elements than expected
func TestParseResponse_ShortRow(t *testing.T) {
	client, sub := setupTestClient(t)

	result := armcostmanagement.QueryResult{
		Properties: &armcostmanagement.QueryProperties{
			Columns: []*armcostmanagement.QueryColumn{
				{Name: stringPtr("Cost"), Type: stringPtr("Number")},
				{Name: stringPtr("UsageDate"), Type: stringPtr("Number")},
				{Name: stringPtr("ServiceName"), Type: stringPtr("String")},
			},
			Rows: [][]interface{}{
				{10.0}, // Missing UsageDate and ServiceName
			},
		},
	}

	records := client.parseResponse(result, sub)

	// Should skip this row because it's missing required UsageDate
	if len(records) != 0 {
		t.Errorf("Expected 0 records for short row, got %d", len(records))
	}
}

// Helper functions

func setupTestClient(t *testing.T) (*Client, config.Subscription) {
	t.Helper()

	cfg := &config.Config{
		Subscriptions: []config.Subscription{
			{ID: "test-sub-1", Name: "test-subscription"},
		},
		Currency: "€",
	}

	client := &Client{
		client: nil, // We don't need actual Azure client for parseResponse tests
		cfg:    cfg,
	}

	sub := cfg.Subscriptions[0]

	return client, sub
}

func loadMockResponse(t *testing.T, filename string) armcostmanagement.QueryResult {
	t.Helper()

	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read test fixture %s: %v", filename, err)
	}

	var apiResponse struct {
		Properties struct {
			Columns []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"columns"`
			Rows [][]interface{} `json:"rows"`
		} `json:"properties"`
	}

	if err := json.Unmarshal(data, &apiResponse); err != nil {
		t.Fatalf("Failed to unmarshal test fixture %s: %v", filename, err)
	}

	// Convert to armcostmanagement.QueryResult format
	result := armcostmanagement.QueryResult{
		Properties: &armcostmanagement.QueryProperties{
			Columns: make([]*armcostmanagement.QueryColumn, len(apiResponse.Properties.Columns)),
			Rows:    apiResponse.Properties.Rows,
		},
	}

	for i, col := range apiResponse.Properties.Columns {
		result.Properties.Columns[i] = &armcostmanagement.QueryColumn{
			Name: stringPtr(col.Name),
			Type: stringPtr(col.Type),
		}
	}

	return result
}
