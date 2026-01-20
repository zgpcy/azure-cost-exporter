# Local Testing with kind

This directory contains configuration for testing the Azure Cost Exporter locally using [kind](https://kind.sigs.k8s.io/) (Kubernetes IN Docker).

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed and running
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- [Helm](https://helm.sh/docs/intro/install/) 3.0+ installed

## Quick Start

Use the Makefile targets for easy testing:

```bash
# Create kind cluster, build and load image, install Helm chart
make kind-test

# Access the exporter
make kind-port-forward

# View logs
make kind-logs

# Clean up
make kind-delete
```

## Manual Steps

### 1. Create kind Cluster

```bash
kind create cluster --config examples/kind/config.yaml --name azure-cost-exporter
```

This creates a single-node Kubernetes cluster with port mappings for local access.

### 2. Build Docker Image

```bash
docker build -t azure-cost-exporter:latest .
```

### 3. Load Image into kind

```bash
kind load docker-image azure-cost-exporter:latest --name azure-cost-exporter
```

### 4. Install Helm Chart

```bash
helm install azure-cost-exporter ./helm/azure-cost-exporter \
  --namespace monitoring \
  --create-namespace \
  -f helm/azure-cost-exporter/test-values.yaml
```

### 5. Verify Deployment

```bash
# Check pod status
kubectl get pods -n monitoring

# Check logs
kubectl logs -f -n monitoring -l app.kubernetes.io/name=azure-cost-exporter

# Check health
kubectl port-forward -n monitoring svc/azure-cost-exporter 8080:8080
curl http://localhost:8080/health
```

### 6. Access the Exporter

```bash
# Port-forward to access locally
kubectl port-forward -n monitoring svc/azure-cost-exporter 8080:8080

# In another terminal, access endpoints:
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/metrics
```

## Configuration

### kind Cluster Configuration

The `config.yaml` file defines:
- Single control-plane node
- Port mappings (8080, 8443)
- Network settings
- Node labels

### Test Values

The `helm/azure-cost-exporter/test-values.yaml` file provides minimal configuration for testing:
- Mock Azure credentials (non-functional)
- Reduced resource requests
- Disabled high-cardinality metrics
- Faster health probes
- Debug logging enabled

## Troubleshooting

### Image Not Found

If you see `ImagePullBackOff` or `ErrImageNeverPull`:

```bash
# Verify image was loaded
docker exec -it azure-cost-exporter-control-plane crictl images | grep azure-cost-exporter

# Reload image if needed
kind load docker-image azure-cost-exporter:latest --name azure-cost-exporter
```

### Pod Not Ready

Check the logs for errors:

```bash
kubectl logs -n monitoring -l app.kubernetes.io/name=azure-cost-exporter
```

Common issues:
- Mock Azure credentials won't authenticate (expected in kind)
- Network connectivity to Azure APIs (expected without real credentials)

### Port Already in Use

If port 8080 is already in use:

```bash
# Use a different port for port-forwarding
kubectl port-forward -n monitoring svc/azure-cost-exporter 8081:8080
curl http://localhost:8081/health
```

## Testing Workflow

1. **Development Loop**:
   ```bash
   # Make code changes
   vim internal/collector/collector.go

   # Rebuild and reload
   make docker-build
   kind load docker-image azure-cost-exporter:latest --name azure-cost-exporter

   # Restart deployment
   kubectl rollout restart deployment/azure-cost-exporter -n monitoring
   kubectl rollout status deployment/azure-cost-exporter -n monitoring
   ```

2. **Test Helm Chart Changes**:
   ```bash
   # Update Helm chart
   vim helm/azure-cost-exporter/values.yaml

   # Upgrade release
   helm upgrade azure-cost-exporter ./helm/azure-cost-exporter \
     --namespace monitoring \
     -f helm/azure-cost-exporter/test-values.yaml
   ```

3. **Test with Real Azure Credentials**:
   ```bash
   # Create secret with real credentials
   kubectl create secret generic azure-credentials \
     --namespace monitoring \
     --from-literal=client-id=<your-client-id> \
     --from-literal=client-secret=<your-client-secret> \
     --from-literal=tenant-id=<your-tenant-id>

   # Create custom values
   cat <<EOF > my-test-values.yaml
   image:
     repository: azure-cost-exporter
     pullPolicy: Never
     tag: "latest"

   existingSecret: "azure-credentials"

   config:
     subscriptions:
       - id: "<your-subscription-id>"
         name: "test"
   EOF

   # Install with real credentials
   helm upgrade azure-cost-exporter ./helm/azure-cost-exporter \
     --namespace monitoring \
     -f my-test-values.yaml
   ```

## Cleanup

```bash
# Delete Helm release
helm uninstall azure-cost-exporter --namespace monitoring

# Delete namespace
kubectl delete namespace monitoring

# Delete kind cluster
kind delete cluster --name azure-cost-exporter
```

Or use the Makefile:

```bash
make kind-delete
```

## CI/CD Integration

You can integrate kind testing into your CI/CD pipeline:

```bash
# Install tools
./scripts/install-kind.sh
./scripts/install-helm.sh

# Run full test
make kind-full-test
```

The `kind-full-test` target:
1. Creates kind cluster
2. Builds and loads image
3. Installs Helm chart
4. Waits for pod ready
5. Tests health endpoints
6. Cleans up cluster

## Additional Resources

- [kind Documentation](https://kind.sigs.k8s.io/)
- [Helm Documentation](https://helm.sh/docs/)
- [kubectl Cheat Sheet](https://kubernetes.io/docs/reference/kubectl/cheatsheet/)
