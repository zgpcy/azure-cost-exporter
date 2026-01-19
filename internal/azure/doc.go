// Package azure provides Azure Cost Management API client functionality.
//
// This package implements a client for querying Azure Cost Management data
// and parsing the results into structured cost records. It handles:
//   - Authentication using Azure Default Credentials
//   - Cost queries with customizable date ranges and grouping dimensions
//   - Response parsing with support for all Azure cost dimensions
//   - Automatic timeout handling for API calls
//
// The main types are:
//   - Client: Azure Cost Management API client
//   - CostRecord: Represents a single cost entry with all dimensions
//   - CostQuerier: Interface for querying costs (useful for testing)
//
// Example usage:
//
//	cfg := &config.Config{
//		Subscriptions: []config.Subscription{
//			{ID: "sub-123", Name: "Production"},
//		},
//		DateRange: config.DateRange{
//			DaysToQuery:   7,
//			EndDateOffset: 1,
//		},
//	}
//
//	client, err := azure.NewClient(cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	records, err := client.QueryCosts(context.Background())
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for _, record := range records {
//		fmt.Printf("Date: %s, Service: %s, Cost: %.2f %s\n",
//			record.Date, record.Service, record.Cost, record.Currency)
//	}
package azure
