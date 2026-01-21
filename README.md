# Azure Cost Exporter

[![Go Version](https://img.shields.io/github/go-mod/go-version/zgpcy/azure-cost-exporter)](https://go.dev/)
[![License](https://img.shields.io/github/license/zgpcy/azure-cost-exporter)](LICENSE)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/azure-cost-exporter)](https://artifacthub.io/packages/search?repo=azure-cost-exporter)

A production-ready Prometheus exporter for Azure Cost Management data, providing daily cost metrics with support for multiple subscriptions and flexible grouping dimensions.

> **Production Ready**: Includes Grafana dashboards, Prometheus recording/alert rules, and Kubernetes manifests. See [examples/](examples/) for complete deployment guides.

## Features

- **Multi-Cloud Ready**: Unified `cloud_cost_daily` metric with `provider` label for Azure, AWS, GCP, and more
- **Time-Series Cost Tracking**: Queries current day's costs at each scrape - Prometheus naturally builds historical trends
- **Multi-Subscription Support**: Query costs across multiple Azure subscriptions
- **Flexible Grouping**: Group costs by resource type, resource group, meter category, and other dimensions
- **Dynamic Labels**: Metric labels are automatically generated based on your groupBy configuration
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

### Running on Kubernetes with Helm

#### Option 1: Install from Helm Repository (Recommended)

Add the Helm repository and install:

```bash
# Add the repository
helm repo add azure-cost-exporter https://zgpcy.github.io/azure-cost-exporter
helm repo update

# Install the chart
helm install azure-cost-exporter azure-cost-exporter/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  --set azure.clientId=<your-client-id> \
  --set azure.clientSecret=<your-client-secret> \
  --set azure.tenantId=<your-tenant-id> \
  --set config.subscriptions[0].id=<subscription-id> \
  --set config.subscriptions[0].name=production
```

#### Option 2: Install from Local Chart

Clone the repository and deploy:

```bash
# Install with Helm
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  --set azure.clientId=<your-client-id> \
  --set azure.clientSecret=<your-client-secret> \
  --set azure.tenantId=<your-tenant-id> \
  --set config.subscriptions[0].id=<subscription-id> \
  --set config.subscriptions[0].name=production
```

For production deployments, it's recommended to use an existing Kubernetes secret:

```bash
# Create secret
kubectl create secret generic azure-credentials \
  --namespace monitoring \
  --from-literal=client-id=<your-client-id> \
  --from-literal=client-secret=<your-client-secret> \
  --from-literal=tenant-id=<your-tenant-id>

# Install chart using existing secret
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  --set existingSecret=azure-credentials \
  --set config.subscriptions[0].id=<subscription-id> \
  --set config.subscriptions[0].name=production
```

See the [Helm chart documentation](helm/azure-cost-exporter/README.md) for more configuration options.

### Local Development with kind

Test locally using kind (Kubernetes IN Docker):

```bash
# Quick start - creates cluster, builds image, installs chart
make kind-test

# Port-forward to access locally
make kind-port-forward

# View logs
make kind-logs

# Clean up
make kind-delete
```

See [kind testing documentation](examples/kind/README.md) for detailed instructions.

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
  end_date_offset: 0   # 0 = today, 1 = yesterday (recommended: 0 for time-series)
  days_to_query: 1     # Query only today (recommended: 1 for time-series tracking)

refresh_interval: 3600  # Refresh data every hour
http_port: 8080
log_level: "info"

group_by:
  enabled: true
  groups:
    - type: Dimension
      name: ResourceType
      label_name: resource_type
    - type: Dimension
      name: ResourceGroup
      label_name: resource_group
    - type: Dimension
      name: MeterCategory
      label_name: meter_category
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

### Available Grouping Dimensions

Configure grouping dimensions to break down costs. Each dimension you add becomes a label in the `cloud_cost_daily` metric:

| Azure Dimension Name | Recommended Label Name | Description |
|---------------------|------------------------|-------------|
| `ServiceName` | `service_name` | Azure service identifier |
| `ResourceType` | `resource_type` | Resource type (e.g., microsoft.compute/virtualmachines) |
| `ResourceGroup` | `resource_group` | Resource group name |
| `ResourceLocation` | `resource_location` | Azure region |
| `ResourceId` | `resource_id` | Full resource identifier (high cardinality!) |
| `MeterCategory` | `meter_category` | Meter category |
| `MeterSubCategory` | `meter_subcategory` | Meter subcategory |
| `ChargeType` | `charge_type` | Usage, Purchase, Refund, etc. |
| `PricingModel` | `pricing_model` | OnDemand, Reservation, Spot, SavingsPlan |

**Important Notes**:
- Use **snake_case** for `label_name` (not PascalCase)
- Adding many dimensions (especially `ResourceId`) significantly increases metric cardinality
- Base labels (`provider`, `account_name`, `account_id`, `service`, `date`, `currency`) are always included

## Authentication

The exporter uses Azure's `DefaultAzureCredential`, which supports multiple authentication methods.

### Recommended: Azure Managed Identity

**Most secure option** - No secrets stored in cluster:

#### 1. Azure Workload Identity (AKS)
Federates Kubernetes ServiceAccount with Azure AD. See [Helm chart README](helm/azure-cost-exporter/README.md#azure-managed-identity-recommended) for setup.

```bash
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --set managedIdentity.enabled=true \
  --set managedIdentity.type=workload-identity \
  --set managedIdentity.clientId=<identity-client-id> \
  --set config.subscriptions[0].id=<subscription-id>
```

#### 2. User-Assigned Managed Identity (VM/VMSS)
```bash
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --set managedIdentity.enabled=true \
  --set managedIdentity.type=user-assigned \
  --set managedIdentity.clientId=<identity-client-id> \
  --set config.subscriptions[0].id=<subscription-id>
```

#### 3. System-Assigned Managed Identity (VM/VMSS)
```bash
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --set managedIdentity.enabled=true \
  --set managedIdentity.type=system-assigned \
  --set config.subscriptions[0].id=<subscription-id>
```

### Alternative: Service Principal with Secret

Using environment variables (for containers without managed identity):

```bash
export AZURE_CLIENT_ID="<your-client-id>"
export AZURE_CLIENT_SECRET="<your-client-secret>"
export AZURE_TENANT_ID="<your-tenant-id>"
```

Or via Helm:
```bash
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --set azure.clientId=<client-id> \
  --set azure.clientSecret=<client-secret> \
  --set azure.tenantId=<tenant-id> \
  --set config.subscriptions[0].id=<subscription-id>
```

### Required Azure Permissions

The service principal or managed identity needs:
- **Role**: `Cost Management Reader` on each subscription
- **Scope**: Subscription level

```bash
# Grant permissions
az role assignment create \
  --assignee <service-principal-or-identity-client-id> \
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

Daily cloud cost with dynamic labels based on your groupBy configuration. Designed for multi-cloud cost monitoring (Azure, AWS, GCP, Cloudflare, etc.).

**Type**: Gauge

**How it works**: The exporter queries today's cost data at each scrape. Prometheus stores this as a time-series, naturally building historical cost trends over days/weeks/months.

**Base Labels** (always present):
- `provider` - Cloud provider (e.g., "azure", "aws", "gcp")
- `account_name` - Subscription/account name from config
- `account_id` - Subscription/account ID
- `service` - Cloud service name (e.g., "Azure DNS", "Virtual Machines")
- `currency` - Currency symbol (e.g., "€")

**Dynamic Labels** (added based on groupBy config):
- Any dimensions you configure (e.g., `resource_type`, `resource_group`, `meter_category`)

**Example with grouping**:
```
cloud_cost_daily{provider="azure",account_name="production",account_id="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",service="Azure DNS",resource_type="microsoft.network/dnszones",resource_group="production-rg",resource_location="westeurope",currency="€"} 0.016
```

**Example without grouping**:
```
cloud_cost_daily{provider="azure",account_name="production",account_id="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",service="Azure DNS",currency="€"} 0.114
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

### Current total cost (all clouds)
```promql
sum(cloud_cost_daily)
```

### Cost by provider
```promql
sum by (provider) (cloud_cost_daily)
```

### Azure cost by account
```promql
sum by (account_name) (cloud_cost_daily{provider="azure"})
```

### Cost by service (all clouds)
```promql
sum by (service, provider) (cloud_cost_daily)
```

### Azure-only current costs
```promql
sum(cloud_cost_daily{provider="azure"})
```

### 7-day cost trend
```promql
sum(cloud_cost_daily) [7d]
```

### Cost over last 24 hours
```promql
sum(cloud_cost_daily[24h])
```

### Daily cost spike detection (Azure) - cost > $100
```promql
cloud_cost_daily{provider="azure"} > 100
```

### Cost rate of change (today vs yesterday)
```promql
delta(sum(cloud_cost_daily)[1d])
```

### Compare Azure vs other clouds (when you have multiple providers)
```promql
sum by (provider) (cloud_cost_daily)
```

### Cost by meter_category (stacked in Grafana)
```promql
sum by (meter_category) (cloud_cost_daily)
```

## Grafana Dashboard

Example Grafana dashboard panels:

**Multi-Cloud Cost Overview (Time Series):**
```json
{
  "targets": [
    {
      "expr": "sum by (provider) (cloud_cost_daily)",
      "legendFormat": "{{provider}}"
    }
  ],
  "title": "Cloud Costs by Provider",
  "type": "timeseries"
}
```

**Azure Cost Breakdown by Service (Stacked):**
```json
{
  "targets": [
    {
      "expr": "sum by (service) (cloud_cost_daily{provider=\"azure\"})",
      "legendFormat": "{{service}}"
    }
  ],
  "title": "Azure Costs by Service",
  "type": "timeseries",
  "stack": "normal"
}
```

**Cost by Meter Category (Stacked - for your use case):**
```json
{
  "targets": [
    {
      "expr": "sum by (meter_category) (cloud_cost_daily)",
      "legendFormat": "{{meter_category}}"
    }
  ],
  "title": "Costs by Meter Category",
  "type": "timeseries",
  "stack": "normal"
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

1. Reduce `days_to_query` in your config
2. Remove high-cardinality dimensions (especially `ResourceId`) from groupBy
3. Disable groupBy entirely by setting `group_by.enabled: false`

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
