# Build stage
FROM golang:1.24.4-alpine AS builder

# Install necessary packages for building
RUN apk add --no-cache git ca-certificates wget

WORKDIR /app

# Copy go mod files first (for better caching)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source code
COPY . .

# Build the binary with optimizations for production
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg/mod CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o activity-service ./microservices/the_monkeys_activity/main.go

# Download grpc_health_probe during build
RUN wget -qO /tmp/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.25/grpc_health_probe-linux-amd64 && \
    chmod +x /tmp/grpc_health_probe

# Production stage - using minimal alpine
FROM alpine:3.19

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Install necessary packages for runtime
RUN apk add --no-cache ca-certificates tzdata curl && \
    update-ca-certificates

# Create necessary directories with proper permissions
RUN mkdir -p /app/logs /app/config && \
    chown -R appuser:appgroup /app

# Copy the binary from builder stage
COPY --from=builder /app/activity-service /app/activity-service
COPY --from=builder /tmp/grpc_health_probe /usr/local/bin/grpc_health_probe

# Copy configuration files if they exist
COPY --chown=appuser:appgroup config/ /app/config/

# Set working directory and user
WORKDIR /app
USER appuser

# Health check using grpc_health_probe
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD grpc_health_probe -addr=:50058 || exit 1

# Expose the service port
EXPOSE 50058

# Run the binary
ENTRYPOINT ["./activity-service"]