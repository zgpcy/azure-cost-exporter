# Contributing to Azure Cost Exporter

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Security](#security)

## Code of Conduct

Be respectful, inclusive, and professional in all interactions.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/azure-cost-exporter.git
   cd azure-cost-exporter
   ```

3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/zgpcy/azure-cost-exporter.git
   ```

## Development Setup

### Prerequisites

- Go 1.22 or later
- Docker (for integration tests and security scanning)
- Make
- Pre-commit hooks (optional but recommended)

### Install Development Tools

```bash
make install-tools
```

This installs:
- golangci-lint (linter)
- gosec (security scanner)
- pre-commit (git hooks)

### Enable Pre-commit Hooks

```bash
pre-commit install
```

This will automatically run checks before each commit:
- Secret detection (gitleaks)
- Code formatting
- YAML validation
- Go tests

## Making Changes

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-number-description
```

Branch naming convention:
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation updates
- `chore/description` - Maintenance tasks

### 2. Make Your Changes

- Write clean, readable code
- Follow Go best practices
- Add tests for new functionality
- Update documentation as needed

### 3. Run Tests Locally

```bash
# Run all CI checks
make ci

# Or run individual checks
make lint      # Linting
make test      # Tests
make security  # Security scans
```

### 4. Commit Your Changes

```bash
git add .
git commit -m "feat: add new feature"
```

**Commit Message Format:**

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `chore`: Maintenance tasks
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `ci`: CI/CD changes

**Examples:**
```
feat(collector): add support for resource tags
fix(config): validate subscription IDs correctly
docs(readme): update installation instructions
```

## Pull Request Process

### 1. Push to Your Fork

```bash
git push origin feature/your-feature-name
```

### 2. Create Pull Request

1. Go to the [main repository](https://github.com/zgpcy/azure-cost-exporter)
2. Click "New Pull Request"
3. Select your fork and branch
4. Fill in the PR template:

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Tests added/updated
- [ ] All tests passing locally
- [ ] Manual testing performed

## Checklist
- [ ] Code follows project style
- [ ] Self-reviewed code
- [ ] Commented complex code
- [ ] Updated documentation
- [ ] No new warnings
- [ ] Added tests
- [ ] All tests pass
- [ ] Security scan passed
```

### 3. Code Review

- Address review comments promptly
- Update your branch if needed
- Keep the PR focused and small if possible

### 4. Merge

Once approved, a maintainer will merge your PR.

## Coding Standards

### Go Style Guide

Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

**Key points:**

1. **Formatting**: Use `gofmt` (enforced by pre-commit)
2. **Naming**: Use clear, descriptive names
3. **Comments**: Document exported functions
4. **Error Handling**: Always check errors
5. **Testing**: Aim for >80% coverage

**Example:**

```go
// QueryCosts retrieves cost data for all configured subscriptions.
// Returns partial data if some subscriptions fail (best-effort approach).
func (c *Client) QueryCosts(ctx context.Context) ([]provider.CostRecord, error) {
    // Implementation
}
```

### Project Structure

```
internal/
├── azure/          # Azure-specific implementation
├── collector/      # Prometheus collector
├── config/         # Configuration handling
├── logger/         # Structured logging
├── provider/       # Cloud provider interface
└── server/         # HTTP server
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test -v ./internal/collector
```

### Test Guidelines

1. **Test files**: `*_test.go` in the same package
2. **Table-driven tests**: Use for multiple scenarios
3. **Mocks**: Use interfaces for testability
4. **Coverage**: Aim for >80% overall

**Example:**

```go
func TestQueryCosts(t *testing.T) {
    tests := []struct {
        name    string
        records []CostRecord
        wantErr bool
    }{
        {
            name: "success",
            records: []CostRecord{{Cost: 100}},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

Coming soon: Integration tests with Azure SDK mocks.

## Security

### Security Checklist

Before submitting a PR:

1. **No secrets in code**
   ```bash
   make security  # Runs gitleaks, gosec, trivy
   ```

2. **No hardcoded credentials**
   - Use environment variables
   - Use `REPLACE_ME` in examples

3. **Dependencies**
   - Run `go mod tidy`
   - Check for vulnerabilities

4. **Sensitive Data**
   - Never log credentials
   - Sanitize error messages

### Reporting Security Issues

See [SECURITY.md](SECURITY.md) for reporting vulnerabilities.

## Questions?

- Open a [GitHub Issue](https://github.com/zgpcy/azure-cost-exporter/issues)
- Check existing issues and PRs first

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
