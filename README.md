<div align="center">

# 🐒 The Monkeys

**A Scalable, Microservices-Based Content Media Platform**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24.4-00ADD8?logo=go)](go.mod)
[![Python Version](https://img.shields.io/badge/Python-3.x-3776AB?logo=python)](requirements.txt)

*Empowering knowledge sharing through modern, distributed architecture*

[Features](#-features) • [Architecture](#-architecture) • [Quick Start](#-quick-start) • [Documentation](#-documentation) • [Contributing](#-contributing)

</div>

---

## 📖 About

**The Monkeys** is a production-ready, microservices-based content media platform designed for knowledge sharing and content creation. Built with modern cloud-native principles, it provides a scalable infrastructure for writers, readers, and experts to share insights across diverse topics including science, technology, personal development, psychology, philosophy, health, and lifestyle.

### Vision

We believe that learning should be accessible to everyone. Our platform is open to anyone who wants to read, learn, and grow. We foster a vibrant community that encourages engagement, conversation, and the free exchange of ideas.

---

## ✨ Features

### Core Capabilities

- **📝 Content Management** - Rich blogging platform with SEO optimization and full-text search
- **👥 User Management** - Comprehensive user profiles, authentication, and social following system
- **🔍 Smart Search** - Elasticsearch-powered content discovery with advanced filtering
- **🤖 AI Recommendations** - ML-based content and user recommendations via dedicated AI engine
- **🔔 Real-time Notifications** - Event-driven notification system with RabbitMQ
- **💾 Object Storage** - Scalable file storage with MinIO (S3-compatible)
- **🔐 Security** - JWT-based authentication, rate limiting, and role-based access control
- **📊 Activity Tracking** - User engagement analytics and behavioral insights
- **🌐 API Gateway** - Centralized HTTP/HTTPS entry point with middleware support

### Technical Highlights

- **Microservices Architecture** - 11+ independent, scalable services
- **gRPC Communication** - High-performance inter-service communication with Protocol Buffers
- **Containerized Deployment** - Docker-based with production-ready configurations
- **Health Monitoring** - Built-in health checks for all services
- **Database Migrations** - Automated schema versioning and migrations
- **Horizontal Scaling** - Stateless services designed for cloud deployment
- **Structured Logging** - Centralized logging with Zap logger

---

## 🏗 Architecture

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              External Clients                               │
│                         (Web, Mobile, Third-party)                          │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │ HTTP/HTTPS
                                 ▼
┌────────────────────────────────────────────────────────────────────────────┐
│                      🌐 API Gateway (Go + Gin)                             │
│           • Authentication Middleware  • Rate Limiting  • Routing          │
└──────┬──────────┬──────────┬──────────┬──────────┬──────────┬──────────────┘
       │          │          │          │          │          │
       │ gRPC     │ gRPC     │ gRPC     │ gRPC     │ gRPC     │ gRPC
       ▼          ▼          ▼          ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│   🔐     │ │   👤     │ │   📝     │ │   💾     │ │   🔔     │ │   📊  │
│  AuthZ   │ │  Users   │ │  Blog    │ │ Storage  │ │ Notifs   │ │ Activity │
│ Service  │ │ Service  │ │ Service  │ │ Service  │ │ Service  │ │ Service  │
└────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘
     │            │            │            │            │            │
     └────────────┴────────────┴────────────┴────────────┴────────────┘
                                 │                        │
                    ┌────────────┼────────────┬──────────┘
                    │            │            │
                    ▼            ▼            ▼
         ┌──────────────┐ ┌─────────────┐ ┌──────────────┐
         │  🤖 AI Engine │ │  💨 Cache   │ │  📡 Stream   │
         │   (Python)    │ │   Service   │ │   Service    │
         │  Recommend.   │ │             │ │              │
         └──────┬────────┘ └─────────────┘ └──────────────┘
                │
                │
┌───────────────┼─────────────────────────────────────────────────────────────┐
│               │              Infrastructure Layer                           │
├───────────────┴─────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐              │
│  │   PostgreSQL    │  │  Elasticsearch  │  │    RabbitMQ     │              │
│  │   Database      │  │   Full-text     │  │  Message Queue  │              │
│  │   (Primary)     │  │   Search        │  │   (Events)      │              │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘              │
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐                                   │
│  │     MinIO       │  │  Auto Migrations│                                   │
│  │  Object Storage │  │   (golang-migrate) │                                │
│  │  (S3-compatible)│  │                 │                                   │
│  └─────────────────┘  └─────────────────┘                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Microservices Overview

| Service | Language | Port | Description |
|---------|----------|------|-------------|
| **Gateway** | Go | 8080 | HTTP/HTTPS API gateway, routing, authentication |
| **AuthZ** | Go | 50051 | Authentication & authorization service |
| **Users** | Go | 50052 | User management, profiles, social features |
| **Blog** | Go | 50053 | Content management, SEO, publishing |
| **Storage** | Go | 50054 | File upload/download, MinIO integration |
| **Notification** | Go | 50055 | Event-driven notifications, RabbitMQ consumer |
| **AI Engine** | Python | 50056 | ML-based recommendations (gRPC) |
| **Activity** | Go | 50057 | User activity tracking and analytics |
| **Cache** | Go | 50058 | Caching layer for performance optimization |
| **Stream** | Go | 50059 | Real-time streaming capabilities |
| **PG** | Go | 50060 | PostgreSQL service wrapper |

---

## 🚀 Quick Start

### Prerequisites

- **Docker** (v20.10+) & **Docker Compose** (v2.0+)
- **Go** (v1.24.4+) - for local development
- **Python** (v3.10+) - for AI engine development
- **Make** - for build automation
- **Git** - version control

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/the-monkeys/the_monkeys_engine.git
   cd the_monkeys_engine
   ```

2. **Configure environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Start the services**
   ```bash
   # Development environment
   docker-compose up -d

   # Production environment
   docker-compose -f prod.docker-compose.yaml up -d
   ```

4. **Verify services are running**
   ```bash
   docker-compose ps
   ```

5. **Access the API**
   - API Gateway: `http://localhost:8080`
   - RabbitMQ Management: `http://localhost:15672`
   - MinIO Console: `http://localhost:9001`

### Health Checks

All services expose health check endpoints:
```bash
# Gateway health
curl http://localhost:8080/healthz

# Individual service health (via Docker)
docker-compose ps
```

---

## ⚙️ Configuration

### Environment Variables

Key configuration variables in `.env`:

```env
# Application
APP_ENV=development

# PostgreSQL
POSTGRESQL_PRIMARY_DB_DB_HOST=the_monkeys_db
POSTGRESQL_PRIMARY_DB_DB_PORT=5432
POSTGRESQL_PRIMARY_DB_DB_USERNAME=postgres
POSTGRESQL_PRIMARY_DB_DB_PASSWORD=your_password
POSTGRESQL_PRIMARY_DB_DB_NAME=the_monkeys

# Elasticsearch
OPENSEARCH_OS_HOST=elasticsearch-node1
OPENSEARCH_HTTP_PORT=9200
OPENSEARCH_OS_PASSWORD=your_password

# RabbitMQ
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_USERNAME=guest
RABBITMQ_PASSWORD=guest

# MinIO
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001

# JWT
JWT_SECRET_KEY=your_secret_key_here
```

### Database Migrations

Migrations are automatically applied on startup. For manual control:

```bash
# Create new migration
make sql-gen

# Apply migrations
make migrate-up

# Rollback migration
make migrate-down

# Force version (use cautiously)
make migrate-force
```

---

## 🛠 Development

### Project Structure

```
the_monkeys_engine/
├── apis/                    # API definitions and service connections
│   └── serviceconn/         # gRPC service connectors
├── microservices/           # Microservice implementations
│   ├── the_monkeys_gateway/     # API Gateway
│   ├── the_monkeys_authz/       # Auth service
│   ├── the_monkeys_users/       # User service
│   ├── the_monkeys_blog/        # Blog service
│   ├── the_monkeys_storage/     # Storage service
│   ├── the_monkeys_notification/# Notification service
│   ├── the_monkeys_ai/          # AI/ML service (Python)
│   ├── the_monkeys_activity/    # Activity service
│   ├── the_monkeys_cache/       # Cache service
│   ├── the_monkeys_stream/      # Stream service
│   └── the_monkeys_pg/          # PostgreSQL service
├── config/                  # Configuration management
├── schema/                  # Database migrations
├── logger/                  # Centralized logging
├── constants/               # Shared constants
├── scripts/                 # Build and deployment scripts
├── docs/                    # Documentation
├── docker-compose.yml       # Development environment
├── prod.docker-compose.yaml # Production environment
├── Makefile                 # Build automation
└── go.mod                   # Go dependencies
```

### Building Locally

```bash
# Generate Protocol Buffers
make proto-gen

# Build all services
docker-compose build

# Build specific service
docker-compose build the_monkeys_gateway

# Run tests
go test ./...
```

### Working with Protocol Buffers

```bash
# Generate Go code from .proto files
make proto-gen

# Generate Python code for AI service
make proto-gen  # Includes Python generation
```

### Local Development (Without Docker)

```bash
# Install Go dependencies
go mod download

# Install Python dependencies (for AI service)
pip install -r requirements.txt

# Run individual service
cd microservices/the_monkeys_gateway
go run main.go
```

---

## 📊 Monitoring & Operations

### Service Resource Limits

Each service is configured with production-ready resource constraints:

- **Memory Limits**: 128MB - 512MB per service
- **CPU Limits**: 0.5 - 1.0 cores per service
- **Health Checks**: 30s intervals with automatic restarts
- **Security**: Non-root user execution (UID 1001)

### Backup & Restore

Automated backup scripts are provided:

```bash
# Restore PostgreSQL database
./restore-postgres.sh    # Linux/macOS
./restore-postgres.ps1   # Windows

# Restore Elasticsearch snapshots
./restore-snapshots.sh   # Linux/macOS
./restore-snapshots.ps1  # Windows
```

### Logging

Structured logging with Zap:

```bash
# View service logs
docker-compose logs -f the_monkeys_gateway

# Set log level (in .env)
LOG_LEVEL=debug  # debug, info, warn, error
```

---

## 📚 Documentation

- **[API Documentation](docs/api/)** - REST API endpoints and examples
- **[Admin API Guide](docs/admin-api.md)** - Administrative endpoints
- **[Deployment Guide](documents/DEPLOYMENT.md)** - Production deployment instructions
- **[GHCR Deployment](documents/GHCR_DEPLOYMENT.md)** - GitHub Container Registry setup
- **[Restore Guide](documents/RESTORE_README.md)** - Backup and restore procedures
- **[Version Management](docs/repo/VERSION_MANAGEMENT.md)** - Release and versioning
- **[Contributing Guidelines](contribution/contribution.md)** - How to contribute

---

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific service tests
cd microservices/the_monkeys_users
go test -v ./...
```

---

## 🐳 Docker Commands

```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# View logs
docker-compose logs -f [service_name]

# Rebuild and restart
docker-compose up -d --build

# Clean up (remove volumes)
docker-compose down -v

# Scale specific service
docker-compose up -d --scale the_monkeys_gateway=3
```

---

## 🔒 Security

- **JWT Authentication**: Secure token-based auth
- **Rate Limiting**: Built-in request throttling
- **Non-root Containers**: All services run as unprivileged users
- **Password Hashing**: bcrypt for secure password storage
- **CORS Configuration**: Configurable cross-origin policies
- **SQL Injection Protection**: Parameterized queries
- **Health Check Isolation**: Internal health endpoints

---

## 🌍 Production Deployment

For production deployments, use the optimized compose file:

```bash
docker-compose -f prod.docker-compose.yaml up -d
```

**Production features:**
- Distroless container images (smaller attack surface)
- Optimized resource allocation
- Automated health checks
- Persistent volume management
- Service dependency orchestration
- Auto-restart policies

See [DEPLOYMENT.md](documents/DEPLOYMENT.md) for detailed production setup.

---

## 🤝 Contributing

We welcome contributions from the community! Whether you're fixing bugs, adding features, or improving documentation, your help is appreciated.

### How to Contribute

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Commit your changes** (`git commit -m 'feat: add amazing feature'`)
4. **Push to the branch** (`git push origin feature/amazing-feature`)
5. **Open a Pull Request**

### Commit Convention

We follow conventional commits:
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Adding tests
- `chore:` - Maintenance tasks

See [Contributing Guidelines](contribution/contribution.md) for more details.

---

## 📄 License

This project is licensed under the **Apache License 2.0** - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- Built with ❤️ by The Monkeys Team
- Powered by modern open-source technologies
- Community-driven development

---

## 📞 Support & Contact

- **Issues**: [GitHub Issues](https://github.com/the-monkeys/the_monkeys_engine/issues)
- **Discussions**: [GitHub Discussions](https://github.com/the-monkeys/the_monkeys_engine/discussions)

---

<div align="center">

**Made with 🐒 by The Monkeys Team**

*Inspire, Inform, Innovate*

</div>
