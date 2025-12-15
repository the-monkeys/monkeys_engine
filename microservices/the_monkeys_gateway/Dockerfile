# Build stage
FROM golang:1.24.4-alpine AS builder

# Install necessary packages for building
RUN apk add --no-cache git ca-certificates

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
    -o gateway-service ./microservices/the_monkeys_gateway/main.go

# Production stage - using minimal alpine
FROM alpine:3.19

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Install ca-certificates, curl, openssh-client, and sshpass for backup operations
RUN apk --no-cache add ca-certificates tzdata curl openssh-client sshpass && \
    rm -rf /var/cache/apk/*

WORKDIR /app

# Create config directory for certificates
RUN mkdir -p config/certs/openssl && \
    chown -R appuser:appgroup config

# Copy the binary from builder stage
COPY --from=builder /app/gateway-service .

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app && \
    chmod +x /app/gateway-service

# Switch to non-root user
USER appuser

# Define port arguments from environment
ARG GATEWAY_HTTP_PORT
ARG GATEWAY_HTTPS_PORT

# Set port environment variables for runtime
ENV GATEWAY_HTTP_PORT=${GATEWAY_HTTP_PORT}
ENV GATEWAY_HTTPS_PORT=${GATEWAY_HTTPS_PORT}

# Expose ports dynamically (HTTP and HTTPS)
EXPOSE ${GATEWAY_HTTP_PORT} ${GATEWAY_HTTPS_PORT}

# Health check using the existing /healthz endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["curl", "-f", "http://localhost:${GATEWAY_HTTP_PORT}/healthz"]

# Run the binary
CMD ["./gateway-service"]