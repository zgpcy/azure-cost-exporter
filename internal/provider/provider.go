package provider

import (
	"context"
)

// ProviderType represents a cloud provider
type ProviderType string

// Supported cloud providers
const (
	ProviderAzure ProviderType = "azure"
	ProviderAWS   ProviderType = "aws"
	ProviderGCP   ProviderType = "gcp"
)

// CloudProvider is the interface that all cloud cost providers must implement
type CloudProvider interface {
	// QueryCosts retrieves cost data from the cloud provider
	QueryCosts(ctx context.Context) ([]CostRecord, error)

	// Name returns the provider name (azure, aws, gcp, etc.)
	Name() ProviderType

	// AccountCount returns the number of accounts/subscriptions being monitored
	AccountCount() int
}

// CostRecord represents a single cost entry from any cloud provider
// This is a generic structure that works across all cloud providers
type CostRecord struct {
	// Common fields across all providers
	Date         string  // YYYY-MM-DD format
	Provider     string  // Cloud provider name (azure, aws, gcp)
	AccountID    string  // Subscription ID (Azure), Account ID (AWS), Project ID (GCP)
	AccountName  string  // Friendly name for the account
	Service      string  // Service name (Storage, Compute, etc.)
	Cost         float64 // Cost amount
	Currency     string  // Currency symbol

	// Optional detailed fields (may be empty for some providers)
	ResourceType     string // Resource type (microsoft.storage/storageaccounts, etc.)
	ResourceGroup    string // Resource group or equivalent organizational unit
	ResourceLocation string // Region/location
	ResourceID       string // Full resource identifier
	ResourceName     string // Resource name

	// Provider-specific metadata
	MeterCategory    string // Azure-specific: Meter category
	MeterSubCategory string // Azure-specific: Meter subcategory
	ChargeType       string // Usage, Purchase, Refund, etc.
	PricingModel     string // OnDemand, Reservation, Spot, etc.
}
