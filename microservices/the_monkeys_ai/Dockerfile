# Build stage
FROM python:3.10-slim as builder

# Install system dependencies needed for building
RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy requirements first for better layer caching
COPY requirements.txt ./
RUN pip install --no-cache-dir --user -r requirements.txt

# Copy source code
COPY . .

# Generate proto files during build
WORKDIR /app/microservices/the_monkeys_ai
RUN python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. gw_recom.proto

# Production stage - using minimal Python slim
FROM python:3.10-slim

# Create non-root user for security
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -s /bin/sh -m appuser

# Install only runtime dependencies
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

WORKDIR /app

# Copy Python packages from builder stage
COPY --from=builder /root/.local /home/appuser/.local

# Copy application code and generated proto files
COPY --from=builder /app .

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Make sure scripts in .local are usable
ENV PATH=/home/appuser/.local/bin:$PATH
ENV PYTHONPATH=/app

# Expose ports (gRPC and health check)
EXPOSE 50057 51057

# Health check using the existing HTTP endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD ["curl", "-f", "http://localhost:51057/health"]

# Run the service
CMD ["python", "-u", "microservices/the_monkeys_ai/main.py"]