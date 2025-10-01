# 🐒 The Monkeys Engine - Docker Compose Guide

## Overview

The Monkeys Engine uses a microservices architecture with Docker Compose for container orchestration. This guide explains the different deployment options and container configurations available.

## Architecture

### Infrastructure Services (Standard Docker Hub Images)
- **PostgreSQL 15** - Primary database
- **RabbitMQ 3.13** - Message broker with management UI
- **Elasticsearch 8.16** - Search and analytics engine
- **MinIO** - S3-compatible object storage

### Custom Microservices (Built from Source)
- **Gateway** - API gateway and routing
- **AI Engine** - AI/ML processing service
- **Blog** - Content management service
- **Auth** - Authentication and authorization
- **User** - User management service
- **Notification** - Notification delivery service
- **Storage** - File storage management

## Deployment Options

### Option 1: Local Development (Build from Source)
```bash
# Build and run all services locally
docker-compose up -d --build

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

**Use Case**: Development, testing, and customization

### Option 2: Production Deployment (Registry Images)
```bash
# Pull pre-built images from GitHub Container Registry
docker-compose -f docker-compose.registry.yml up -d

# Or combine with local infrastructure
docker-compose up -d postgres rabbitmq elasticsearch minio
docker-compose -f docker-compose.registry.yml up -d gateway ai-engine blog auth user notification storage
```

**Use Case**: Production deployments, faster startup times

### Option 3: Hybrid Deployment
```bash
# Run infrastructure locally, use registry for microservices
docker-compose up -d the_monkeys_db rabbitmq elasticsearch-node1 minio
# Then deploy specific microservices from registry as needed
```

**Use Case**: Testing specific microservice updates

## Configuration

### Environment Variables
Ensure your `.env` file is properly configured:

```bash
# Copy example configuration
cp .env.example .env

# Edit configuration values
nano .env
```

### Key Configuration Sections

#### Database Configuration
```env
POSTGRESQL_PRIMARY_DB_DB_HOST=the_monkeys_db
POSTGRESQL_PRIMARY_DB_DB_PORT=5432
POSTGRESQL_PRIMARY_DB_INTERNAL_PORT=5432
POSTGRESQL_PRIMARY_DB_DB_USERNAME=postgres
POSTGRESQL_PRIMARY_DB_DB_PASSWORD=your_secure_password
POSTGRESQL_PRIMARY_DB_DB_NAME=the_monkeys_db
```

#### Message Queue Configuration
```env
RABBITMQ_DEFAULT_USER=admin
RABBITMQ_DEFAULT_PASS=your_secure_password
RABBITMQ_PORT=5672
RABBITMQ_MANAGEMENT_PORT=15672
```

#### Storage Configuration
```env
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=your_secure_password
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
```

## Service Health Checks

All services include health checks for reliable deployments:

- **Database**: `pg_isready` command
- **RabbitMQ**: `rabbitmq-diagnostics ping`
- **Elasticsearch**: HTTP health endpoint
- **MinIO**: HTTP health endpoint
- **Microservices**: Custom health endpoints

## Port Configuration

### Infrastructure Services
- PostgreSQL: `5432`
- RabbitMQ: `5672` (AMQP), `15672` (Management UI)
- Elasticsearch: `9200`
- MinIO: `9000` (API), `9001` (Console)

### Microservices
- Gateway: `8081`
- AI Engine: `50057` (gRPC), `51600` (Health)
- Blog: `50051`
- Auth: `50052`
- User: `50053`
- Notification: `50054`
- Storage: `50055`

## Networking

All services communicate through the `monkeys-network` bridge network, enabling:
- Service discovery by container name
- Isolated network communication
- Load balancing and scaling capabilities

## Volume Management

### Persistent Data Volumes
- `postgres_data` - Database storage
- `rabbitmq_data` - Message queue data
- `elasticsearch_data` - Search indices
- `minio_data` - Object storage

### Backup Volumes
- `./backup` - Database backup location
- `./postgres_backup` - PostgreSQL backup source
- `./elasticsearch_snapshots` - Elasticsearch snapshots

## Scaling and Load Balancing

### Horizontal Scaling
```bash
# Scale specific microservices
docker-compose up -d --scale blog=3 --scale user=2

# Scale with registry images
docker-compose -f docker-compose.registry.yml up -d --scale blog=3
```

### Resource Limits
Configure resource limits in compose files:
```yaml
deploy:
  resources:
    limits:
      cpus: '0.5'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 256M
```

## Monitoring and Logging

### Service Logs
```bash
# View all logs
docker-compose logs -f

# View specific service logs
docker-compose logs -f gateway

# View last 100 lines
docker-compose logs --tail=100 ai-engine
```

### Health Status
```bash
# Check service status
docker-compose ps

# Check health of specific service
docker inspect <container_name> | grep Health -A 10
```

## Troubleshooting

### Common Issues

#### Port Conflicts
```bash
# Check port usage
netstat -tulpn | grep :8081

# Change ports in .env file
THE_MONKEYS_GATEWAY_HTTP_PORT=8082
```

#### Database Connection Issues
```bash
# Check database logs
docker-compose logs the_monkeys_db

# Test connection
docker-compose exec the_monkeys_db psql -U postgres -d the_monkeys_db
```

#### Memory Issues
```bash
# Check resource usage
docker stats

# Increase Elasticsearch memory
ES_JAVA_OPTS=-Xms2g -Xmx2g
```

### Service Dependencies

Services start in dependency order:
1. Infrastructure services (DB, RabbitMQ, Elasticsearch, MinIO)
2. Core services (Auth, User)
3. Application services (Blog, Notification, Storage)
4. Gateway (last, depends on all services)

## Security Considerations

### Network Security
- Services communicate through internal network
- Only necessary ports exposed to host
- Health checks use internal endpoints

### Authentication
- Database and RabbitMQ require authentication
- MinIO uses access keys
- Microservices run as non-root user (1001:1001)

### Environment Variables
- Never commit `.env` file to version control
- Use strong passwords for all services
- Rotate credentials regularly

## CI/CD Integration

### GitHub Actions Workflow
The project includes automated building and publishing:
- Builds microservices on every push to main
- Publishes to GitHub Container Registry (ghcr.io)
- Generates registry deployment configuration
- Supports multi-architecture builds (AMD64/ARM64)

### Registry Images
```bash
# Available pre-built images
ghcr.io/the-monkeys/monkeys-gateway:latest
ghcr.io/the-monkeys/monkeys-ai-engine:latest
ghcr.io/the-monkeys/monkeys-blog:latest
ghcr.io/the-monkeys/monkeys-auth:latest
ghcr.io/the-monkeys/monkeys-user:latest
ghcr.io/the-monkeys/monkeys-notification:latest
ghcr.io/the-monkeys/monkeys-storage:latest
```

## Quick Start Commands

```bash
# 🚀 Quick development setup
git clone <repository>
cd the_monkeys_engine
cp .env.example .env
# Edit .env with your values
docker-compose up -d --build

# 🏭 Quick production setup
git clone <repository>
cd the_monkeys_engine
cp .env.example .env
# Edit .env with your values
docker-compose -f docker-compose.registry.yml up -d

# 🔧 Maintenance commands
docker-compose down              # Stop all services
docker-compose down -v           # Stop and remove volumes
docker system prune -a           # Clean up unused containers/images
```

## Support

For issues and questions:
- Check service logs: `docker-compose logs <service>`
- Verify configuration: `docker-compose config`
- Test connectivity: `docker-compose exec <service> <command>`
- Review health status: `docker-compose ps`

---

🐒 **The Monkeys Engine Team**  
*Making microservices deployment simple and reliable*