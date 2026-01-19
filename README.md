# Azure Cost Exporter

[![Go Version](https://img.shields.io/github/go-mod/go-version/zgpcy/azure-cost-exporter)](https://go.dev/)
[![License](https://img.shields.io/github/license/zgpcy/azure-cost-exporter)](LICENSE)

A production-ready Prometheus exporter for Azure Cost Management data, providing daily cost metrics with support for multiple subscriptions and flexible grouping dimensions.

> **Production Ready**: Includes Grafana dashboards, Prometheus recording/alert rules, and Kubernetes manifests. See [examples/](examples/) for complete deployment guides.

## Features

- **Multi-Cloud Ready**: Unified `cloud_cost_daily` metric with `provider` label for Azure, AWS, GCP, and more
- **Daily Cost Breakdown**: Exposes costs as time-series data with daily granularity
- **Multi-Subscription Support**: Query costs across multiple Azure subscriptions
- **Flexible Grouping**: Group costs by ServiceName, ResourceType, MeterCategory, and other dimensions
- **Cardinality Control**: Toggle high-cardinality metrics to optimize Prometheus memory usage
- **Structured Logging**: JSON-formatted logs with configurable levels (debug, info, warn, error)
- **Background Refresh**: Periodically queries Azure API to minimize rate limiting
- **Production Ready**:
  - Comprehensive test coverage (97.5% on collector, 71.7% overall)
  - Grafana dashboard for cost visualization
  - Prometheus recording and alerting rules
  - Kubernetes manifests with health/readiness probes
- **Configuration**: YAML file + environment variable overrides
- **Lightweight**: Small Docker image based on Alpine Linux

## Multi-Cloud Design

This exporter uses a unified metric name `cloud_cost_daily` with a `provider="azure"` label instead of `azure_cost_daily`. This design makes it easy to:

- **Add other cloud providers**: Deploy AWS, GCP, or Cloudflare cost exporters using the same metric name
- **Unified dashboards**: Single Grafana dashboard for all cloud costs
- **Cross-cloud queries**: Compare costs across providers with simple PromQL
- **Consistent schema**: Same label structure across all clouds

**Example future expansion:**
```promql
cloud_cost_daily{provider="azure"}      # This exporter
cloud_cost_daily{provider="aws"}        # Future AWS exporter
cloud_cost_daily{provider="gcp"}        # Future GCP exporter
cloud_cost_daily{provider="cloudflare"} # Future Cloudflare exporter
```

## Quick Start

### Prerequisites

- Go 1.22+ (for building from source)
- Docker (for containerized deployment)
- Azure subscription with Cost Management API access
- Azure credentials configured (see [Authentication](#authentication))

### Running with Docker

```bash
# Build the image
docker build -t azure-cost-exporter:latest .

# Run the container
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -e AZURE_CLIENT_ID=<your-client-id> \
  -e AZURE_CLIENT_SECRET=<your-client-secret> \
  -e AZURE_TENANT_ID=<your-tenant-id> \
  azure-cost-exporter:latest
```

### Running Locally

```bash
# Install dependencies
go mod download

# Build the binary
go build -o azure-cost-exporter ./cmd/exporter

# Run the exporter
./azure-cost-exporter -config config.yaml
```

## Configuration

### Configuration File

The exporter uses a YAML configuration file (`config.yaml`):

```yaml
subscriptions:
  - id: "31193c31-7631-4120-990b-dfb31478f7da"
    name: "production"
  - id: "bff44dec-916c-4139-b390-43e93fb04593"
    name: "development"

currency: "€"

date_range:
  end_date_offset: 0   # 0 = today, 1 = yesterday
  days_to_query: 7     # Number of days to fetch

refresh_interval: 3600  # Refresh data every hour
http_port: 8080
log_level: "info"

group_by:
  enabled: true
  groups:
    - type: Dimension
      name: ServiceName
      label_name: ServiceName
    - type: Dimension
      name: ResourceType
      label_name: ResourceType
```

### Environment Variables

Configuration values can be overridden with environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `AZURE_COST_SUBSCRIPTIONS` | Comma-separated subscription list: `id1:name1,id2:name2` | From config file |
| `AZURE_COST_CURRENCY` | Currency symbol for display | `€` |
| `AZURE_COST_REFRESH_INTERVAL` | Refresh interval in seconds | `3600` |
| `AZURE_COST_HTTP_PORT` | HTTP server port | `8080` |
| `AZURE_COST_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `AZURE_COST_END_DATE_OFFSET` | Days before today for end date | `0` |
| `AZURE_COST_DAYS_TO_QUERY` | Number of days to query | `7` |
| `AZURE_COST_ENABLE_HIGH_CARDINALITY_METRICS` | Enable resource-level metrics (`true`/`false`) | `true` |

### Available Grouping Dimensions

You can group costs by any of these Azure Cost Management dimensions:

- `ServiceName` - Azure service (e.g., Microsoft.Compute)
- `ResourceType` - Resource type (e.g., virtualMachines)
- `ResourceGroup` - Resource group name
- `ResourceLocation` - Azure region
- `MeterCategory` - Meter category
- `MeterSubCategory` - Meter subcategory
- `SubscriptionId` - Subscription ID
- `ChargeType` - Usage, Purchase, Refund, etc.
- `PricingModel` - OnDemand, Reservation, Spot, SavingsPlan

**Note**: Azure allows up to 2 grouping dimensions per query.

## Authentication

The exporter uses Azure's `DefaultAzureCredential`, which supports multiple authentication methods in this order:

1. **Environment Variables** (recommended for containers):
   ```bash
   export AZURE_CLIENT_ID="<your-client-id>"
   export AZURE_CLIENT_SECRET="<your-client-secret>"
   export AZURE_TENANT_ID="<your-tenant-id>"
   ```

2. **Managed Identity** (recommended for Azure VMs/AKS)
3. **Azure CLI** (for local development)

### Required Azure Permissions

The service principal or managed identity needs:
- **Role**: `Cost Management Reader` on each subscription
- **Scope**: Subscription level

```bash
# Grant permissions
az role assignment create \
  --assignee <service-principal-id> \
  --role "Cost Management Reader" \
  --scope "/subscriptions/<subscription-id>"
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/` | Landing page with exporter status and links |
| `/metrics` | Prometheus metrics endpoint |
| `/health` | Health check (liveness probe) - always returns 200 |
| `/ready` | Readiness check - returns 200 only when data is loaded |

## Metrics

### `cloud_cost_daily`

Daily cloud cost by provider, subscription, service, and resource type. Designed for multi-cloud cost monitoring (Azure, AWS, GCP, Cloudflare, etc.).

**Type**: Gauge
**Labels**:
- `provider` - Cloud provider (e.g., "azure", "aws", "gcp", "cloudflare")
- `subscription` - Subscription/account name from config
- `subscription_id` - Subscription/account ID
- `service` - Cloud service name (e.g., "Microsoft.Compute", "Amazon S3")
- `resource_type` - Resource type or meter category
- `date` - Date in YYYY-MM-DD format
- `currency` - Currency symbol (e.g., "€")

**Example**:
```
cloud_cost_daily{provider="azure",subscription="production",subscription_id="31193c31-7631-4120-990b-dfb31478f7da",service="Microsoft.Compute",resource_type="Virtual Machines",date="2026-01-16",currency="€"} 45.67
```

### `azure_cost_exporter_up`

Exporter health status.

**Type**: Gauge
**Values**:
- `1` - Last Azure query successful
- `0` - Last Azure query failed

## Prometheus Configuration

Add this job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'azure-cost'
    scrape_interval: 5m  # Don't scrape too frequently
    static_configs:
      - targets: ['azure-cost-exporter:8080']
```

## Production Deployment

For production deployments, see the comprehensive examples in the [`examples/`](examples/) directory:

- **[Grafana Dashboard](examples/grafana/)** - Ready-to-import dashboard with cost visualizations
- **[Prometheus Rules](examples/prometheus/)** - Recording rules for pre-aggregated metrics
- **[Alert Rules](examples/alerts/)** - Cost anomaly detection and health monitoring alerts
- **[Kubernetes Manifests](examples/kubernetes/)** - Production-ready K8s deployment with:
  - ConfigMap with full configuration options
  - Deployment with health/readiness probes
  - ServiceMonitor for Prometheus Operator
  - Secret templates for Azure credentials

> **Quick Start**: See [examples/README.md](examples/README.md) for complete deployment instructions, best practices, and troubleshooting guides.

## Kubernetes Deployment

### Basic Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: azure-cost-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: azure-cost-exporter
  template:
    metadata:
      labels:
        app: azure-cost-exporter
    spec:
      containers:
      - name: exporter
        image: azure-cost-exporter:latest
        ports:
        - containerPort: 8080
          name: metrics
        env:
        - name: AZURE_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: azure-credentials
              key: client-id
        - name: AZURE_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: azure-credentials
              key: client-secret
        - name: AZURE_TENANT_ID
          valueFrom:
            secretKeyRef:
              name: azure-credentials
              key: tenant-id
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
          readOnly: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
      volumes:
      - name: config
        configMap:
          name: azure-cost-exporter-config
---
apiVersion: v1
kind: Service
metadata:
  name: azure-cost-exporter
  labels:
    app: azure-cost-exporter
spec:
  ports:
  - port: 8080
    targetPort: 8080
    name: metrics
  selector:
    app: azure-cost-exporter
```

### ServiceMonitor (Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: azure-cost-exporter
spec:
  selector:
    matchLabels:
      app: azure-cost-exporter
  endpoints:
  - port: metrics
    interval: 5m
    path: /metrics
```

## Example PromQL Queries

### Total cost today (all clouds)
```promql
sum(cloud_cost_daily{date="2026-01-16"})
```

### Cost by provider
```promql
sum by (provider) (cloud_cost_daily{date="2026-01-16"})
```

### Azure cost by subscription
```promql
sum by (subscription) (cloud_cost_daily{provider="azure",date="2026-01-16"})
```

### Cost by service (all clouds)
```promql
sum by (service, provider) (cloud_cost_daily{date="2026-01-16"})
```

### Azure-only costs
```promql
sum(cloud_cost_daily{provider="azure",date="2026-01-16"})
```

### 7-day cost trend
```promql
sum(cloud_cost_daily{date=~"2026-01-(10|11|12|13|14|15|16)"})
```

### Daily cost spike detection (Azure)
```promql
cloud_cost_daily{provider="azure"} > 100
```

### Compare today vs yesterday
```promql
sum(cloud_cost_daily{date="2026-01-16"}) - sum(cloud_cost_daily{date="2026-01-15"})
```

### Compare Azure vs other clouds (when you have multiple providers)
```promql
sum by (provider) (cloud_cost_daily{date="2026-01-16"})
```

## Grafana Dashboard

Example Grafana dashboard panels:

**Multi-Cloud Cost Overview:**
```json
{
  "targets": [
    {
      "expr": "sum by (date, provider) (cloud_cost_daily)",
      "legendFormat": "{{provider}} - {{date}}"
    }
  ],
  "title": "Daily Cloud Costs by Provider"
}
```

**Azure-Specific Cost Breakdown:**
```json
{
  "targets": [
    {
      "expr": "sum by (date, service) (cloud_cost_daily{provider=\"azure\"})",
      "legendFormat": "{{service}} - {{date}}"
    }
  ],
  "title": "Azure Daily Costs by Service"
}
```

## Troubleshooting

### Exporter not ready

Check logs:
```bash
kubectl logs -l app=azure-cost-exporter
```

Common issues:
- Azure authentication failure - verify credentials
- Permission denied - ensure Cost Management Reader role
- Invalid subscription IDs - check config.yaml

### No metrics appearing

1. Check exporter status: `curl http://localhost:8080/`
2. Verify readiness: `curl http://localhost:8080/ready`
3. Check Prometheus targets: Prometheus UI -> Status -> Targets
4. Verify date range settings in config.yaml

### High memory usage

Reduce `days_to_query` or disable some grouping dimensions.

## Development

### Building

```bash
# Build binary
go build -o azure-cost-exporter ./cmd/exporter

# Run tests with coverage
go test -race -cover ./...

# Build Docker image
docker build -t azure-cost-exporter:latest .
```

### Testing

```bash
# Run all tests with race detection and coverage
go test -race -cover ./...

# Run tests for specific package
go test -v ./internal/collector

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Test Coverage:**
- Collector: 97.5%
- Server: 71.7%
- Config: 78.3%
- Azure Client: 51.5%

### Testing locally

```bash
# Set Azure credentials
export AZURE_CLIENT_ID="..."
export AZURE_CLIENT_SECRET="..."
export AZURE_TENANT_ID="..."

# Run with config file
./azure-cost-exporter -config config.yaml

# Override config with environment variables
AZURE_COST_LOG_LEVEL=debug ./azure-cost-exporter -config config.yaml
```

### Project Structure

```
.
 cmd/
    exporter/
        main.go              # Application entry point
 internal/
    azure/
       cost_client.go       # Azure Cost Management API client
    config/
       config.go            # Configuration handling
    collector/
       cost_collector.go    # Prometheus collector
    server/
        server.go            # HTTP server
 config.yaml                   # Configuration file
 Dockerfile                    # Container image
 README.md                     # This file
```

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.
