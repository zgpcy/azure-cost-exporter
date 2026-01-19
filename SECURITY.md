# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via:

1. **GitHub Security Advisories** (preferred):
   - Go to https://github.com/zgpcy/azure-cost-exporter/security/advisories
   - Click "Report a vulnerability"

2. **Email**:
   - Send details to: [your-security-email@example.com]
   - Include "SECURITY" in the subject line

### What to Include

Please include the following information:

- Type of vulnerability
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Release**: Depends on severity (Critical: 7 days, High: 14 days, Medium: 30 days)

## Security Best Practices

### For Users

1. **Never commit secrets to version control**
   - Use Kubernetes Secrets, Azure Key Vault, or similar
   - Enable pre-commit hooks to prevent accidental commits

2. **Use least-privilege access**
   - Grant only "Cost Management Reader" role
   - Never use Owner or Contributor roles

3. **Keep dependencies updated**
   - Regularly update to the latest version
   - Monitor security advisories

4. **Enable security scanning**
   - Use Trivy or similar to scan Docker images
   - Enable Dependabot alerts

5. **Secure your deployment**
   - Use Network Policies in Kubernetes
   - Enable Pod Security Standards
   - Use Azure Workload Identity (not service principals with secrets)

### For Contributors

1. **Run security scans before submitting PRs**
   ```bash
   make security
   pre-commit run --all-files
   ```

2. **Never include real credentials in code or tests**
   - Use mocks and fake data
   - Use "REPLACE_ME" placeholders in examples

3. **Keep dependencies minimal and audited**
   - Review new dependencies carefully
   - Check for known vulnerabilities

## Security Features

This project includes:

- ✅ Pre-commit hooks with secret detection (gitleaks)
- ✅ Automated vulnerability scanning (Trivy, Gosec)
- ✅ Dependency scanning (GitHub Dependabot)
- ✅ CodeQL security analysis
- ✅ Docker image scanning
- ✅ SARIF upload to GitHub Security

## Known Security Considerations

1. **Azure API Credentials**
   - This exporter requires Azure credentials to query the Cost Management API
   - Follow the principle of least privilege
   - Use Managed Identity or Workload Identity when possible

2. **Metrics Exposure**
   - Cost data is exposed via `/metrics` endpoint
   - Implement authentication/authorization at the ingress level
   - Consider using mTLS for Prometheus scraping

3. **Memory Usage**
   - Large cost datasets can consume significant memory
   - Set appropriate resource limits in Kubernetes
   - Use `enable_high_cardinality_metrics: false` for large environments

## Acknowledgments

We appreciate responsible disclosure and will acknowledge security researchers who report vulnerabilities to us.
