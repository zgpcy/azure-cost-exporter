# Release Process

## Helm Chart Publishing

This project uses GitHub Actions to automatically publish Helm charts to GitHub Pages and Artifact Hub.

### Prerequisites

1. **Enable GitHub Pages**:
   - Go to repository Settings → Pages
   - Source: Deploy from a branch
   - Branch: `gh-pages` → `/` (root)
   - Click Save

2. **Ensure GitHub Actions permissions**:
   - Go to repository Settings → Actions → General
   - Workflow permissions: "Read and write permissions"
   - Check "Allow GitHub Actions to create and approve pull requests"

### Release Steps

1. **Update Chart Version**:
   ```bash
   # Edit helm/azure-cost-exporter/Chart.yaml
   # Bump version: 0.1.0 -> 0.2.0
   # Bump appVersion if code changed: main -> v0.2.0
   ```

2. **Commit Changes**:
   ```bash
   git add helm/azure-cost-exporter/Chart.yaml
   git commit -m "Bump chart version to 0.2.0"
   git push origin main
   ```

3. **Create Git Tag**:
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```

4. **Automated Process**:
   - GitHub Actions workflow triggers on tag push
   - Packages Helm chart
   - Creates GitHub Release
   - Updates `gh-pages` branch with chart package
   - Updates Helm repository index

5. **Verify Release**:
   - Check GitHub Actions workflow status
   - Verify GitHub Release created
   - Test Helm repository:
     ```bash
     helm repo add azure-cost-exporter https://zgpcy.github.io/azure-cost-exporter
     helm repo update
     helm search repo azure-cost-exporter
     ```

### Register on Artifact Hub (One-time)

1. Go to https://artifacthub.io
2. Sign in with GitHub
3. Click "Control Panel" → "Add Repository"
4. Fill in:
   - **Name**: azure-cost-exporter
   - **Display name**: Azure Cost Exporter
   - **URL**: https://zgpcy.github.io/azure-cost-exporter
   - **Type**: Helm charts
   - **Official**: ✓ (if you're the maintainer)
5. Click "Add"

Artifact Hub will automatically sync your chart within 30 minutes.

### Manual Release (Fallback)

If GitHub Actions fails, you can release manually:

```bash
# Package chart
helm package helm/azure-cost-exporter -d .deploy

# Checkout gh-pages branch
git checkout gh-pages

# Move package and update index
mv .deploy/*.tgz .
helm repo index . --url https://zgpcy.github.io/azure-cost-exporter

# Commit and push
git add .
git commit -m "Release chart version X.Y.Z"
git push origin gh-pages

# Return to main branch
git checkout main
```

## Versioning Strategy

Follow Semantic Versioning (SemVer):

- **MAJOR** (X.0.0): Breaking changes (configuration, API changes)
- **MINOR** (0.X.0): New features, backward compatible
- **PATCH** (0.0.X): Bug fixes, documentation updates

### Chart vs App Versioning

- `version` in Chart.yaml: Helm chart version
- `appVersion` in Chart.yaml: Application/container version

Example:
- Chart version `0.2.0` might package app version `v1.3.0`
- Update both if application code changed
- Update only chart version if only Helm templates changed
