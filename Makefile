.PHONY: help build test lint security clean install-tools

# Variables
BINARY_NAME=azure-cost-exporter
VERSION?=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(shell go version | awk '{print $$3}')

# Build flags
LDFLAGS=-ldflags "-X github.com/zgpcy/azure-cost-exporter/internal/version.Version=${VERSION} \
                  -X github.com/zgpcy/azure-cost-exporter/internal/version.BuildTime=${BUILD_TIME} \
                  -X github.com/zgpcy/azure-cost-exporter/internal/version.GoVersion=${GO_VERSION}"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	pip3 install pre-commit
	pre-commit install

build: ## Build the binary
	@echo "Building ${BINARY_NAME} ${VERSION}..."
	go build ${LDFLAGS} -o ${BINARY_NAME} ./cmd/exporter

build-linux: ## Build for Linux (useful for Docker)
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY_NAME}-linux ./cmd/exporter

test: ## Run tests with coverage
	@echo "Running tests..."
	go test -race -cover -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

test-verbose: ## Run tests with verbose output
	go test -v -race -cover ./...

test-coverage: ## Generate HTML coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run Go linters
	@echo "Running golangci-lint..."
	golangci-lint run --timeout 5m
	@echo "Running go vet..."
	go vet ./...
	@echo "Running go fmt check..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "Code is not formatted. Run 'make fmt'"; \
		exit 1; \
	fi

fmt: ## Format Go code
	@echo "Formatting code..."
	gofmt -s -w .
	go mod tidy

security: ## Run security scans
	@echo "Running security scans..."
	@echo "1. Checking for secrets with gitleaks..."
	docker run --rm -v ${PWD}:/repo zricethezav/gitleaks:latest detect --source /repo --no-git
	@echo "2. Running gosec (Go security checker)..."
	gosec -quiet ./...
	@echo "3. Running Trivy for vulnerabilities..."
	docker run --rm -v ${PWD}:/app aquasec/trivy:latest fs --severity HIGH,CRITICAL /app
	@echo "4. Checking dependencies..."
	go list -json -m all | docker run --rm -i sonatypecommunity/nancy:latest sleuth

docker-build: ## Build Docker image
	docker build -t ${BINARY_NAME}:${VERSION} .
	docker tag ${BINARY_NAME}:${VERSION} ${BINARY_NAME}:latest

docker-scan: ## Scan Docker image for vulnerabilities
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest image ${BINARY_NAME}:latest

run: build ## Build and run locally
	./${BINARY_NAME} -config config.yaml

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f ${BINARY_NAME} ${BINARY_NAME}-linux
	rm -f coverage.out coverage.html
	go clean

pre-commit: ## Run pre-commit hooks on all files
	pre-commit run --all-files

ci: lint test security ## Run all CI checks (lint, test, security)
	@echo "✅ All CI checks passed!"

release-check: ## Pre-release checks
	@echo "Running pre-release checks..."
	@echo "1. Checking git status..."
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "❌ Working directory is not clean"; \
		exit 1; \
	fi
	@echo "2. Running tests..."
	@make test
	@echo "3. Running security scans..."
	@make security
	@echo "4. Building binary..."
	@make build
	@echo "✅ Ready for release!"

.DEFAULT_GOAL := help
