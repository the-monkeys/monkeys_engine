# Monkeys Engine Docker Guide

This guide is the single source of truth for running the **Monkeys Engine** microservices stack.

## 🚀 Quick Start

### Prerequisites
*   Docker & Docker Compose (v2.20+)
*   Make (Optional, but recommended)

### 1-Command Startup (Development)
```bash
# copy env if not exists
cp .env.example .env

# Start everything in Development mode
docker compose -f docker-compose-dev.yml up -d
```
*   **Access the Gateway**: `http://localhost:8081`
*   **Hot Reload**: Edit files locally, and they update in the container instantly.

---

## 🏗️ Architecture Overview

We use a split-configuration strategy to optimize for both *Developer Experience* and *Production Performance*.

| File | Purpose | Key Features |
| :--- | :--- | :--- |
| **`docker-compose-dev.yml`** | **Development** | • **DRY / Optimized**: Uses YAML anchors for clean, shared settings.<br>• **Hot-Reload**: Source code mounted via volumes.<br>• **Debuggable**: All internal ports exposed. |
| **`docker-compose-prod.yml`** | **Production** | • **Secure**: Internal ports hidden; only Gateway exposed.<br>• **Immutable**: Code is built into images; no volume mounts.<br>• **Efficient**: Lowest resource overhead. |
| **`docker-compose.yml`** | **Manual / Standard** | • **Legacy Structure**: No YAML anchors; settings are repeated manually.<br>• **Granular**: Easier to tweak single service settings without affecting others.<br>• **Standard**: Uses standard YAML syntax familiar to all developers. |

---

## 🛠️ Usage Guide

### Starting Services
| Environment | Command |
| :--- | :--- |
| **Development** | `docker compose -f docker-compose-dev.yml up` |
| **Production** | `docker compose -f docker-compose-prod.yml up --build` |
| **original** | `docker compose -f docker-compose.yml up --build` |

### Viewing Logs
Tail logs for a specific service (e.g., Gateway):
```bash
docker compose -f docker-compose-dev.yml logs -f the_monkeys_gateway
```

### Rebuilding (Dev)
If you add a new dependency (`go mod`), you must rebuild:
```bash
# Rebuild only the specific service for speed
docker compose -f docker-compose-dev.yml up -d --build --no-deps the_monkeys_gateway
```

### Teardown
To stop and remove containers/networks:
```bash
docker compose -f docker-compose-dev.yml down
```

---

## ⚙️ Configuration

### Environment Variables
*   All configuration is driven by the `.env` file.
*   **Validation**: If `docker compose config` fails, check if `.env` is missing variables.

### Port Reference (Localhost)
In **Development**, all these ports are exposed. In **Production**, ONLY the Gateway (8081) is open.

| Service | Internal Port | Localhost Port | Description |
| :--- | :--- | :--- | :--- |
| **Gateway** | 8081 | `8081` | Main HTTP Entrypoint |
| **Authz** | 50051 | `50051` | gRPC Auth Service |
| **Blog** | 50052 | `50052` | gRPC Blog Service |
| **Users** | 50053 | `50053` | gRPC User Service |
| **Storage** | 50054 | `50054` | gRPC Storage Service |
| **Activity** | 50058 | `50058` | gRPC Activity Service |
| **RabbitMQ** | 15672 | `15672` | Management UI (User: `guest`/`guest`) |
| **MinIO** | 9001 | `9001` | Console UI |
| **Postgres** | 5432 | `5432` | Database Direct Access |

---

## 📊 Performance Benchmarks

Why use `prod` config for deployment? It is significantly more efficient.
*(Benchmarks run on local machine)*

| Metric | Production (`prod`) | Development (`dev`) | Original (`current`) | Improvement |
| :--- | :--- | :--- | :--- | :--- |
| **Startup Time (Warm)** | **12.8s** | **9.1s** | **~9s** | Dev/Original are identical in structure. |
| **MinIO RAM** | **84.5 MB** | 152.4 MB | 77.9 MB | Prod/Original are more efficient. |
| **Microservices RAM** | **~6.7 MB** | ~10.6 MB | ~10-32 MB | Prod binaries are the most efficient. |
| **Elasticsearch RAM** | **1.47 GB** | 1.81 GB | 1.56 GB | Prod setup is the most optimized. |

---

## ❓ Troubleshooting

### 1. `invalid proto` error
*   **Cause**: Missing `.env` file. Docker interprets empty variables as invalid configuration.
*   **Fix**: `cp .env.example .env`

### 2. Port Conflicts (`address already in use`)
*   **Cause**: Another service (or an old zombie container) is using port 5432, 8081, etc.
*   **Fix**: `docker stop $(docker ps -q)` or check system processes `lsof -i :8081`.

### 3. Changes not reflecting (Production)
*   **Cause**: Prod uses **Images**, not volume mounts.
*   **Fix**: You MUST rebuild the image: `docker compose -f docker-compose-prod.yml up -d --build`.

---

## 🏗️ Dockerfile Architecture

We maintain a consistent, secure, and optimized build strategy across all microservices.

### Common Structure
All microservices (`gateway`, `authz`, `users`, etc.) follow this **Multi-Stage Build** pattern:

1.  **Builder Stage** (`FROM golang:1.24-alpine`)
    *   Downloads dependencies (`go mod download`) with **cache mounts** for speed.
    *   Compiles the binary with optimization flags (`-w -s` to shrink size).
    *   Downloads `grpc_health_probe` (except for Gateway which uses `curl`).
2.  **Runner Stage** (`FROM alpine:3.19`)
    *   **Minimal Base**: Uses Alpine Linux (~5MB) for a tiny footprint.
    *   **Security**: Creates a non-root `appuser (1001)` and switches to it.
    *   **Dependencies**: Installs only runtime needs (ca-certificates, tzdata).
    *   **Artifacts**: Copies *only* the compiled binary and health probe from the Builder stage.

### Service-Specific Details

| Service | Dockerfile Path | Port | Key Features |
| :--- | :--- | :--- | :--- |
| **Gateway** | `microservices/the_monkeys_gateway/Dockerfile` | `8080`, `8081` | • Installs `curl`, `openssh-client`, `sshpass`.<br>• Creates `config/certs/openssl` for TLS.<br>• Healthcheck via HTTP `/healthz`. |
| **Authz** | `microservices/the_monkeys_authz/Dockerfile` | `50051` | • Standard gRPC setup.<br>• Healthcheck via `grpc_health_probe`. |
| **Storage** | `microservices/the_monkeys_storage/Dockerfile` | `50054` | • **Data Directories**: Creates `profile`, `blogs` folders for file storage.<br>• Installs `wget` (builder) and `ca-certificates` (runner). |
| **Users** | `microservices/the_monkeys_users/Dockerfile` | `50053` | • Standard gRPC setup.<br>• Healthcheck via `grpc_health_probe`. |
| **Blog** | `microservices/the_monkeys_blog/Dockerfile` | `50052` | • Standard gRPC setup.<br>• Healthcheck via `grpc_health_probe`. |
| **Activity** | `microservices/the_monkeys_activity/Dockerfile` | `50058` | • Creates `/app/logs` directory.<br>• Copies `config/` directory.<br>• Strict `ENTRYPOINT` definition. |

