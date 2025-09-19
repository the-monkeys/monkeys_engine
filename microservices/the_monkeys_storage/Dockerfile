# Build stage
FROM golang:1.24.4-alpine AS builder

# Install necessary packages for building
RUN apk add --no-cache git ca-certificates wget

WORKDIR /app

# Copy go mod files first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations for production
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o storage-service ./microservices/the_monkeys_storage/main.go

# Download grpc_health_probe during build
RUN wget -qO /tmp/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.25/grpc_health_probe-linux-amd64 && \
    chmod +x /tmp/grpc_health_probe

# Production stage - using minimal alpine
FROM alpine:3.19

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

WORKDIR /app

# Create directories for storage (matching volume mounts)
RUN mkdir -p /profile /blogs && \
    chown -R appuser:appgroup /profile /blogs

# Copy the binary and health probe from builder stage
COPY --from=builder /app/storage-service .
COPY --from=builder /tmp/grpc_health_probe /usr/local/bin/grpc_health_probe

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app && \
    chmod +x /app/storage-service

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 50054

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/grpc_health_probe", "-addr=localhost:50054"]

# Run the binary
CMD ["./storage-service"]