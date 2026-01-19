# Azure Cost Exporter - Examples

This directory contains production-ready examples for deploying and monitoring the Azure Cost Exporter.

## Directory Structure

```
examples/
├── grafana/              # Grafana dashboard
├── prometheus/           # Prometheus recording rules
├── kubernetes/           # Kubernetes manifests
└── alerts/              # Prometheus alert rules
```

## Quick Start

### 1. Kubernetes Deployment

Deploy the exporter to your Kubernetes cluster:

```bash
# Create namespace
kubectl create namespace monitoring

# Create Azure credentials secret (edit the file first!)
kubectl apply -f kubernetes/azure-credentials-secret.yaml

# Deploy the exporter
kubectl apply -f kubernetes/deployment.yaml

# Verify deployment
kubectl -n monitoring get pods -l app=azure-cost-exporter
kubectl -n monitoring logs -l app=azure-cost-exporter -f
```

### 2. Prometheus Configuration

Add recording and alert rules to Prometheus:

```yaml
# Add to your prometheus.yml
rule_files:
  - "recording-rules.yaml"
  - "cost-alerts.yaml"
```

```bash
# Copy rules to Prometheus
cp prometheus/recording-rules.yaml /etc/prometheus/
cp alerts/cost-alerts.yaml /etc/prometheus/

# Reload Prometheus
curl -X POST http://localhost:9090/-/reload
```

### 3. Grafana Dashboard

Import the dashboard into Grafana:

**Option A: Via UI**
1. Open Grafana → Dashboards → Import
2. Upload `grafana/azure-cost-dashboard.json`
3. Select your Prometheus datasource
4. Click "Import"

**Option B: Via API**
```bash
curl -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @grafana/azure-cost-dashboard.json
```

## Configuration Guide

### Kubernetes

**Key configuration points:**

1. **Azure Credentials** (`kubernetes/azure-credentials-secret.yaml`):
   - Create a Service Principal with `Cost Management Reader` role
   - Encode credentials to base64
   - For production, use Workload Identity (recommended) or sealed-secrets

2. **Config Map** (`kubernetes/deployment.yaml`):
   - Update `subscriptions` with your Azure subscription IDs
   - Adjust `refresh_interval` based on your needs (default: 1 hour)
   - Set `enable_high_cardinality_metrics: false` if you have cardinality concerns

3. **Resources**:
   - Default: 100m CPU / 128Mi RAM (requests)
   - Adjust based on number of subscriptions and resources

### Prometheus

**Recording Rules** (`prometheus/recording-rules.yaml`):

Pre-aggregated metrics for better performance:
- `azure:cost:total_daily` - Total daily cost
- `azure:cost:by_subscription` - Cost per subscription
- `azure:cost:by_service` - Cost per service
- `azure:cost:7d_total` - 7-day cost total
- `azure:cost:30d_avg` - 30-day average

**Alert Rules** (`alerts/cost-alerts.yaml`):

Cost monitoring alerts:
- `AzureDailyCostHigh` - Daily cost exceeds threshold
- `AzureCostSpike` - >20% day-over-day increase
- `AzureCostExporterDown` - Exporter health check
- `AzureCostDataStale` - Data not updated recently

**Customizing Thresholds:**

Edit the alert expressions to match your environment:
```yaml
# Example: Change daily cost threshold from €1000 to €5000
- alert: AzureDailyCostHigh
  expr: azure:cost:total_daily > 5000  # Changed from 1000
```

### Grafana

**Dashboard Features:**

1. **Overview Panels**:
   - Total daily cost
   - Exporter health status
   - Record count and scrape duration

2. **Visualizations**:
   - Cost trends over time (stacked by service)
   - Service distribution (donut chart)
   - Subscription distribution (donut chart)
   - Detailed cost breakdown table

3. **Customization**:
   - Adjust time range (default: last 7 days)
   - Modify currency in panel settings
   - Add filters by subscription or service

## Production Best Practices

### Security

1. **Credentials Management**:
   ```bash
   # Use Workload Identity (AKS)
   az aks update -g <rg> -n <cluster> --enable-workload-identity

   # Or use external-secrets
   kubectl apply -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml
   ```

2. **RBAC**:
   - Minimal permissions: `Cost Management Reader` on subscriptions
   - Never use `Owner` or `Contributor` roles

3. **Network Policies**:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: azure-cost-exporter
   spec:
     podSelector:
       matchLabels:
         app: azure-cost-exporter
     policyTypes:
       - Ingress
       - Egress
     ingress:
       - from:
           - podSelector:
               matchLabels:
                 app: prometheus
         ports:
           - port: 8080
     egress:
       - to:
           - namespaceSelector: {}
         ports:
           - port: 53  # DNS
       - to:
           - podSelector: {}
         ports:
           - port: 443  # Azure API
   ```

### Performance

1. **Cardinality Control**:
   ```yaml
   # Disable high-cardinality metrics if you have >10k resources
   enable_high_cardinality_metrics: false
   ```

2. **Resource Tuning**:
   ```yaml
   resources:
     requests:
       cpu: 200m       # Increase for many subscriptions
       memory: 256Mi   # Increase if caching >100k records
     limits:
       cpu: 1000m
       memory: 1Gi
   ```

3. **Scrape Intervals**:
   ```yaml
   # Prometheus ServiceMonitor
   interval: 60s        # Cost data doesn't change frequently
   scrapeTimeout: 30s   # Allow time for Azure API calls
   ```

### Monitoring

**Key Metrics to Watch:**

```promql
# Exporter health
up{job="azure-cost-exporter"}

# Scrape duration (should be <60s)
cloud_cost_exporter_scrape_duration_seconds

# Data freshness (should be <2h)
time() - cloud_cost_exporter_last_scrape_timestamp_seconds

# Error rate
rate(cloud_cost_exporter_scrape_errors_total[5m])
```

## Troubleshooting

### Exporter Not Starting

```bash
# Check logs
kubectl -n monitoring logs -l app=azure-cost-exporter --tail=100

# Common issues:
# 1. Invalid Azure credentials
# 2. Missing subscriptions config
# 3. Network connectivity to Azure API
```

### No Metrics in Prometheus

```bash
# Verify ServiceMonitor
kubectl -n monitoring get servicemonitor azure-cost-exporter

# Check Prometheus targets
curl http://prometheus:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="azure-cost-exporter")'

# Verify exporter /metrics endpoint
kubectl -n monitoring port-forward svc/azure-cost-exporter 8080:8080
curl http://localhost:8080/metrics
```

### High Memory Usage

```yaml
# Reduce cardinality
enable_high_cardinality_metrics: false

# Reduce query range
date_range:
  days_to_query: 3  # Instead of 7

# Increase memory limits
resources:
  limits:
    memory: 1Gi  # Instead of 512Mi
```

### Slow Scrapes

```bash
# Check Azure API latency
kubectl -n monitoring logs -l app=azure-cost-exporter | grep "duration_seconds"

# Solutions:
# 1. Increase api_timeout in config
# 2. Reduce number of subscriptions per instance
# 3. Increase scrapeTimeout in ServiceMonitor
```

## Support

For issues and questions:
- GitHub Issues: [Your repo URL]
- Logs: `kubectl -n monitoring logs -l app=azure-cost-exporter -f`
- Metrics: http://your-exporter:8080/metrics
