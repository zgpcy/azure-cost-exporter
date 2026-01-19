# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o azure-cost-exporter \
    ./cmd/exporter

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 exporter && \
    adduser -D -u 1000 -G exporter exporter

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/azure-cost-exporter /app/azure-cost-exporter

# Copy default config example (can be overridden by volume mount)
COPY config.yaml.example /app/config.yaml

# Set ownership
RUN chown -R exporter:exporter /app

# Switch to non-root user
USER exporter

# Expose metrics port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the exporter
ENTRYPOINT ["/app/azure-cost-exporter"]
CMD ["-config", "/app/config.yaml"]
