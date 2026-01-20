# Azure Cost Exporter Helm Chart

Helm chart for deploying the Azure Cost Exporter to Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Azure Service Principal with Cost Management Reader role (or Azure Workload Identity configured)

## Installation

### Add the Repository (if published)

```bash
helm repo add azure-cost-exporter https://zgpcy.github.io/azure-cost-exporter
helm repo update
```

### Install from Local Chart

```bash
# Clone the repository
git clone https://github.com/zgpcy/azure-cost-exporter.git
cd azure-cost-exporter

# Install the chart
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  --set azure.clientId=<your-client-id> \
  --set azure.clientSecret=<your-client-secret> \
  --set azure.tenantId=<your-tenant-id> \
  --set config.subscriptions[0].id=<subscription-id> \
  --set config.subscriptions[0].name=production
```

### Using a Values File

Create a `my-values.yaml` file:

```yaml
azure:
  clientId: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  clientSecret: "your-secret-value"
  tenantId: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

config:
  subscriptions:
    - id: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
      name: "production"
    - id: "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy"
      name: "development"

  currency: "€"

  dateRange:
    endDateOffset: 1
    daysToQuery: 7

  refreshInterval: 3600
  logLevel: "info"

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

Install with your values:

```bash
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  -f my-values.yaml
```

## Using Existing Secret (Recommended)

Instead of passing credentials via values, use an existing Kubernetes secret:

```bash
# Create the secret
kubectl create secret generic azure-credentials \
  --namespace monitoring \
  --from-literal=client-id=<your-client-id> \
  --from-literal=client-secret=<your-client-secret> \
  --from-literal=tenant-id=<your-tenant-id>

# Install the chart referencing the secret
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  --set existingSecret=azure-credentials \
  --set config.subscriptions[0].id=<subscription-id> \
  --set config.subscriptions[0].name=production
```

## Azure Managed Identity (Recommended)

**Azure Managed Identity is the most secure authentication method** - no secrets are stored in the cluster.

### Option 1: Azure Workload Identity (Recommended for AKS)

Azure Workload Identity federates Kubernetes ServiceAccounts with Azure AD identities.

```bash
# 1. Enable Workload Identity on AKS
az aks update -g <rg> -n <cluster> --enable-workload-identity --enable-oidc-issuer

# 2. Create Managed Identity
az identity create -g <rg> -n azure-cost-exporter-identity

# 3. Assign Cost Management Reader role
az role assignment create \
  --assignee $(az identity show -g <rg> -n azure-cost-exporter-identity --query clientId -o tsv) \
  --role "Cost Management Reader" \
  --scope "/subscriptions/<subscription-id>"

# 4. Create federated credential
az identity federated-credential create \
  --name azure-cost-exporter-fed-cred \
  --identity-name azure-cost-exporter-identity \
  --resource-group <rg> \
  --issuer $(az aks show -g <rg> -n <cluster> --query "oidcIssuerProfile.issuerUrl" -o tsv) \
  --subject system:serviceaccount:monitoring:azure-cost-exporter \
  --audience api://AzureADTokenExchange
```

Install the chart:

```bash
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  -f workload-identity-values.yaml
```

See [workload-identity-values.yaml](workload-identity-values.yaml) for complete example.

### Option 2: User-Assigned Managed Identity (VM/VMSS)

For non-AKS deployments on Azure VMs or VM Scale Sets:

```bash
# 1. Create and attach identity
az identity create -g <rg> -n azure-cost-exporter-identity
az vm identity assign -g <rg> -n <vm-name> \
  --identities $(az identity show -g <rg> -n azure-cost-exporter-identity --query id -o tsv)

# 2. Assign permissions
az role assignment create \
  --assignee $(az identity show -g <rg> -n azure-cost-exporter-identity --query clientId -o tsv) \
  --role "Cost Management Reader" \
  --scope "/subscriptions/<subscription-id>"
```

See [user-assigned-managed-identity-values.yaml](user-assigned-managed-identity-values.yaml) for values.

### Option 3: System-Assigned Managed Identity (Simplest)

Uses the VM/VMSS's built-in system identity:

```bash
# 1. Enable system identity
az vm identity assign -g <rg> -n <vm-name>

# 2. Assign permissions
az role assignment create \
  --assignee $(az vm identity show -g <rg> -n <vm-name> --query principalId -o tsv) \
  --role "Cost Management Reader" \
  --scope "/subscriptions/<subscription-id>"
```

See [system-assigned-managed-identity-values.yaml](system-assigned-managed-identity-values.yaml) for values.

## Configuration

### Key Parameters

#### Authentication

| Parameter | Description | Default |
|-----------|-------------|---------|
| `managedIdentity.enabled` | Enable Azure Managed Identity authentication | `false` |
| `managedIdentity.type` | Type: `workload-identity`, `user-assigned`, `system-assigned` | `workload-identity` |
| `managedIdentity.clientId` | Managed Identity Client ID (required for workload-identity/user-assigned) | `""` |
| `existingSecret` | Name of existing secret for Service Principal credentials | `""` |
| `azure.clientId` | Service Principal Client ID (only when managedIdentity.enabled=false) | `""` |
| `azure.clientSecret` | Service Principal Client Secret (only when managedIdentity.enabled=false) | `""` |
| `azure.tenantId` | Azure Tenant ID (only when managedIdentity.enabled=false) | `""` |

#### General

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas (should be 1) | `1` |
| `image.repository` | Image repository | `ghcr.io/zgpcy/azure-cost-exporter` |
| `image.tag` | Image tag | `main` |
| `config.subscriptions` | List of Azure subscriptions to monitor | `[]` |
| `config.currency` | Currency symbol for cost metrics | `"€"` |
| `config.dateRange.endDateOffset` | Days to offset from today | `1` |
| `config.dateRange.daysToQuery` | Number of days to query | `7` |
| `config.refreshInterval` | How often to refresh cost data (seconds) | `3600` |
| `config.logLevel` | Log level (debug, info, warn, error) | `"info"` |
| `config.enableHighCardinalityMetrics` | Enable resource-level metrics | `true` |
| `serviceMonitor.enabled` | Enable Prometheus Operator ServiceMonitor | `false` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |

### Full Configuration

See [values.yaml](values.yaml) for all available configuration options.

## Upgrading

```bash
helm upgrade azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall azure-cost-exporter --namespace monitoring
```

## Prometheus Integration

### Using Prometheus Operator

Enable the ServiceMonitor:

```yaml
serviceMonitor:
  enabled: true
  additionalLabels:
    prometheus: kube-prometheus
  interval: 60s
  scrapeTimeout: 30s
```

### Manual Prometheus Configuration

Add this to your Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: 'azure-cost-exporter'
    static_configs:
      - targets: ['azure-cost-exporter.monitoring.svc.cluster.local:8080']
```

## Troubleshooting

### View Logs

```bash
kubectl logs -f -n monitoring -l app.kubernetes.io/name=azure-cost-exporter
```

### Check Health

```bash
kubectl port-forward -n monitoring svc/azure-cost-exporter 8080:8080
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/metrics
```

### Common Issues

1. **Pod is not ready**: Check if Azure credentials are correct and subscriptions exist
2. **No metrics**: Verify Azure Cost Management data is available (usually 1-day delay)
3. **High memory usage**: Disable high-cardinality metrics by setting `config.enableHighCardinalityMetrics: false`

## Examples

See the [examples](../../examples/) directory for:
- Grafana dashboards
- Prometheus recording rules
- Alert rules
- Kubernetes manifests

## Development

For local testing with kind, see [DEVELOPMENT.md](../../DEVELOPMENT.md).

## License

MIT

## Support

For issues and questions, please open an issue at https://github.com/zgpcy/azure-cost-exporter/issues
