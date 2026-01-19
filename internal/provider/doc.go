// Package provider defines the cloud provider abstraction layer.
//
// This package provides a generic interface for querying cost data from
// different cloud providers (Azure, AWS, GCP, etc.). It allows the exporter
// to support multiple cloud providers without changing the core collector logic.
//
// The CloudProvider interface must be implemented by each cloud-specific package:
//
//	type CloudProvider interface {
//		QueryCosts(ctx context.Context) ([]CostRecord, error)
//		Name() ProviderType
//		AccountCount() int
//	}
//
// The CostRecord structure is designed to work across all cloud providers,
// with common fields that all providers must populate and optional fields
// for provider-specific details:
//
// Common fields (always populated):
//   - Date, Provider, AccountID, AccountName, Service, Cost, Currency
//
// Optional fields (provider-specific):
//   - ResourceType, ResourceGroup, ResourceLocation, ResourceID, ResourceName
//   - MeterCategory, MeterSubCategory, ChargeType, PricingModel
//   - Tags (for custom dimensions)
//
// Example implementation:
//
//	type AzureProvider struct {
//		client *armcostmanagement.QueryClient
//		config *config.Config
//	}
//
//	func (p *AzureProvider) QueryCosts(ctx context.Context) ([]provider.CostRecord, error) {
//		// Azure-specific implementation
//		// Convert Azure API response to provider.CostRecord
//	}
//
//	func (p *AzureProvider) Name() provider.ProviderType {
//		return provider.ProviderAzure
//	}
//
//	func (p *AzureProvider) AccountCount() int {
//		return len(p.config.Subscriptions)
//	}
//
// This abstraction allows:
//   - Supporting multiple cloud providers simultaneously
//   - Easy addition of new cloud providers
//   - Provider-agnostic metric collection
//   - Consistent cost data structure across clouds
package provider
