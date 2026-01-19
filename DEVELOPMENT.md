# Development Guide

Complete guide for developing and maintaining Azure Cost Exporter.

## Quick Start

```bash
# 1. Install development tools
make install-tools

# 2. Run all checks locally
make ci

# 3. Run the exporter
make run
```

## Available Make Commands

```bash
make help                # Show all available commands
make build              # Build binary
make test               # Run tests with coverage
make test-coverage      # Generate HTML coverage report
make lint               # Run linters
make fmt                # Format code
make security           # Run security scans
make docker-build       # Build Docker image
make docker-scan        # Scan Docker image
make pre-commit         # Run pre-commit hooks
make ci                 # Run all CI checks
make release-check      # Pre-release validation
```

## Security Best Practices

### 1. Pre-commit Hooks

Automatically run before each commit:

```bash
# Install hooks
pre-commit install

# Run manually
pre-commit run --all-files
```

**What it checks:**
- ✅ Secret detection (gitleaks)
- ✅ Code formatting (gofmt)
- ✅ YAML validation
- ✅ Large files
- ✅ Private keys
- ✅ Tests pass

### 2. Security Scanning Tools

#### Gitleaks (Secret Detection)
```bash
# Local scan
docker run --rm -v ${PWD}:/repo zricethezav/gitleaks:latest detect --source /repo --no-git

# What it detects:
# - API keys
# - Passwords
# - Tokens
# - Private keys
# - Connection strings
```

#### Gosec (Go Security)
```bash
# Run gosec
gosec ./...

# What it checks:
# - SQL injection
# - Command injection
# - Weak crypto
# - Unsafe file operations
```

#### Trivy (Vulnerability Scanner)
```bash
# Scan filesystem
docker run --rm -v ${PWD}:/app aquasec/trivy:latest fs --severity HIGH,CRITICAL /app

# Scan Docker image
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy:latest image azure-cost-exporter:latest

# What it finds:
# - CVEs in dependencies
# - Vulnerable OS packages
# - Configuration issues
```

#### Nancy (Dependency Checker)
```bash
# Check Go dependencies
go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth

# What it checks:
# - Known vulnerabilities in dependencies
# - License issues
```

## CI/CD Pipeline

### GitHub Actions Workflows

#### 1. CI Workflow (`.github/workflows/ci.yml`)

Runs on every push and PR:

```yaml
Jobs:
  - Lint (golangci-lint)
  - Test (with coverage)
  - Security Scan (Trivy + Gosec)
  - Build (multiple platforms)
```

**Triggers:**
- Push to `main` or `develop`
- Pull requests to `main`

**What it does:**
1. Runs golangci-lint with strict rules
2. Executes all tests with race detection
3. Uploads coverage to Codecov
4. Runs Trivy and Gosec security scans
5. Uploads SARIF results to GitHub Security tab
6. Builds binaries for Linux/Darwin (AMD64/ARM64)
7. Uploads artifacts

#### 2. Docker Workflow (`.github/workflows/docker.yml`)

Builds and scans Docker images:

```yaml
Jobs:
  - Build Docker image
  - Scan with Trivy
  - Push to ghcr.io (on main/tags)
```

**Triggers:**
- Push to `main`
- Tags `v*`
- Pull requests

**What it does:**
1. Builds Docker image with BuildKit
2. Scans for vulnerabilities
3. Uploads results to GitHub Security
4. Pushes to GitHub Container Registry (on main)

#### 3. CodeQL Workflow (`.github/workflows/codeql.yml`)

Advanced security analysis:

**Triggers:**
- Push to `main`
- Pull requests
- Weekly schedule (Mondays 6 AM)

**What it does:**
- Static analysis of Go code
- Detects:
  - SQL injection
  - Command injection
  - Path traversal
  - XSS vulnerabilities
  - And 200+ other security issues

### Dependabot (`.github/dependabot.yml`)

Automatic dependency updates:

```yaml
Updates:
  - Go modules (weekly)
  - GitHub Actions (weekly)
  - Docker base images (weekly)
```

**What it does:**
- Creates PRs for dependency updates
- Labels PRs appropriately
- Requests review from maintainers
- Runs CI checks automatically

## Linting Configuration

### golangci-lint (`.golangci.yml`)

**Enabled Linters:**
- `errcheck` - Unchecked errors
- `gosimple` - Code simplification
- `govet` - Standard Go checks
- `staticcheck` - Static analysis
- `gosec` - Security issues
- `gofmt` - Code formatting
- `goimports` - Import organization
- `misspell` - Spelling errors
- `gocritic` - Comprehensive checks
- `revive` - Golint replacement

**Custom Rules:**
- Local imports: `github.com/zgpcy/azure-cost-exporter`
- Test files: Relaxed rules
- Max issues per linter: 50

## Testing Strategy

### Unit Tests

**Coverage Requirements:**
- Overall: >70%
- Collector: >95%
- Server: >70%
- Config: >75%

```bash
# Run with coverage
go test -race -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out
```

### Test Structure

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   Input
        want    Output
        wantErr bool
    }{
        {
            name:    "success case",
            input:   validInput,
            want:    expectedOutput,
            wantErr: false,
        },
        {
            name:    "error case",
            input:   invalidInput,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Feature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Mocking

Use interfaces for testability:

```go
// Provider interface allows mocking
type CloudProvider interface {
    QueryCosts(ctx context.Context) ([]CostRecord, error)
    Name() ProviderType
}

// Mock implementation for tests
type mockProvider struct {
    records []CostRecord
    err     error
}
```

## Release Process

### 1. Pre-release Checks

```bash
# Run comprehensive checks
make release-check
```

Checks:
- ✅ Working directory clean
- ✅ All tests pass
- ✅ Security scans pass
- ✅ Binary builds successfully

### 2. Version Tagging

```bash
# Create tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push tag
git push origin v1.0.0
```

### 3. Automated Release

GitHub Actions automatically:
1. Builds binaries for all platforms
2. Creates Docker images
3. Scans for vulnerabilities
4. Creates GitHub Release
5. Publishes to ghcr.io

## Secret Management

### DO NOT Commit:
- ❌ API keys
- ❌ Passwords
- ❌ Tokens
- ❌ Private keys
- ❌ Subscription IDs (real ones)
- ❌ Connection strings

### Safe Practices:
- ✅ Use environment variables
- ✅ Use `REPLACE_ME` in examples
- ✅ Run pre-commit hooks
- ✅ Use `.gitignore` for sensitive files
- ✅ Use Kubernetes Secrets
- ✅ Use Azure Key Vault

### Files to Exclude:

Already in `.gitignore`:
```
config.yaml
*-secret.yaml
*.key
*.pem
.env
EXPERT_REVIEW.md
```

## Troubleshooting

### Pre-commit Hook Failures

```bash
# Skip hooks temporarily (not recommended)
git commit --no-verify

# Fix formatting
make fmt

# Run specific hook
pre-commit run gitleaks --all-files
```

### Test Failures

```bash
# Verbose output
go test -v ./...

# Run single test
go test -run TestSpecificFunction ./internal/collector

# Race detection
go test -race ./...
```

### Linter Errors

```bash
# Auto-fix what can be fixed
golangci-lint run --fix

# Ignore specific line (use sparingly)
//nolint:linter-name // Explanation why
```

### Docker Build Issues

```bash
# Clear build cache
docker builder prune

# Build without cache
docker build --no-cache -t azure-cost-exporter:latest .
```

## Performance Optimization

### Profiling

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=.

# Memory profile
go test -memprofile=mem.prof -bench=.

# View profile
go tool pprof cpu.prof
```

### Benchmarks

```go
func BenchmarkQueryCosts(b *testing.B) {
    for i := 0; i < b.N; i++ {
        QueryCosts(context.Background())
    }
}
```

## Resources

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [golangci-lint](https://golangci-lint.run/)
- [GitHub Security](https://docs.github.com/en/code-security)
- [Trivy](https://aquasecurity.github.io/trivy/)

## Getting Help

- [GitHub Issues](https://github.com/zgpcy/azure-cost-exporter/issues)
- [Discussions](https://github.com/zgpcy/azure-cost-exporter/discussions)
- [Security](SECURITY.md)
