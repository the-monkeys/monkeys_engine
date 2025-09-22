# 🚀 The Monkeys Engine Deployment Guide

Complete step-by-step deployment instructions for The Monkeys Engine microservices architecture with optimized containers and health monitoring.

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Development Deployment](#development-deployment)
4. [Production Deployment](#production-deployment)
5. [Health Monitoring](#health-monitoring)
6. [Troubleshooting](#troubleshooting)
7. [Scaling & Optimization](#scaling--optimization)

## 🔧 Prerequisites

### Required Software
- **Docker** 20.10+ with Docker Compose
- **Git** for repository management
- **PowerShell** 5.1+ (Windows) or **Bash** 4.0+ (Linux/macOS)

### Required Resources
- **RAM**: Minimum 8GB, Recommended 16GB+
- **Storage**: 20GB+ free space
- **CPU**: 4+ cores recommended
- **Network**: Internet access for image pulls

### Environment Files
```bash
# Copy and configure environment
cp .env.example .env
# Edit .env with your configuration
```

## ⚙️ Environment Setup

### 1. Clone and Navigate
```bash
git clone https://github.com/the-monkeys/the_monkeys_engine.git
cd the_monkeys_engine
```

### 2. Configure Environment Variables
Edit `.env` file with your settings:

```properties
# Essential Configuration
APP_ENV=production
LOG_LEVEL=info
LOG_FORMAT=json

# Database Configuration
POSTGRESQL_PRIMARY_DB_DB_USERNAME=your_username
POSTGRESQL_PRIMARY_DB_DB_PASSWORD=your_password
POSTGRESQL_PRIMARY_DB_DB_NAME=the_monkeys_user_prod

# JWT Configuration
JWT_SECRET_KEY=your_secure_jwt_secret_key_here

# Email Configuration
EMAIL_SMTP_ADDRESS=smtp.your-provider.com:587
EMAIL_SMTP_MAIL=your-email@domain.com
EMAIL_SMTP_PASSWORD=your-email-password
```

### 3. Create Required Directories
```bash
# Create log directories
mkdir -p logs
mkdir -p local_profiles local_blogs
mkdir -p postgres_backup elasticsearch_snapshots
```

## 🔨 Development Deployment

### Quick Start (5 minutes)
```bash
# 1. Build and start all services
docker-compose up --build -d

# 2. Verify health status
docker ps

# 3. Check logs if needed
docker-compose logs -f
```

### Service-by-Service Build
```bash
# Build individual services
docker-compose build the_monkeys_authz
docker-compose build the_monkeys_blog
docker-compose build the_monkeys_user
docker-compose build the_monkeys_storage
docker-compose build the_monkeys_notification
docker-compose build the_monkeys_gateway
docker-compose build the_monkeys_ai_engine

# Start infrastructure first
docker-compose up -d the_monkeys_db elasticsearch-node1 rabbitmq the_monkeys_cache minio

# Start microservices
docker-compose up -d the_monkeys_authz the_monkeys_blog the_monkeys_user
docker-compose up -d the_monkeys_storage the_monkeys_notification
docker-compose up -d the_monkeys_gateway the_monkeys_ai_engine
```

### Development Environment Features
- **Hot reload** enabled for code changes
- **Debug logging** with console output
- **Local volumes** for data persistence
- **Network isolation** between services

## 🏭 Production Deployment

### 1. Production Environment Setup
```bash
# Use production compose file
export COMPOSE_FILE=docker-compose.prod.yml

# Or specify explicitly
docker-compose -f docker-compose.prod.yml up -d
```

### 2. Production Build Process
```bash
# Build with distroless variants for maximum security
docker-compose -f docker-compose.prod.yml build --no-cache

# Verify images
docker images | grep the_monkeys
```

### 3. Production Scaling
```bash
# Scale critical services
docker-compose -f docker-compose.prod.yml up -d --scale the_monkeys_authz=3
docker-compose -f docker-compose.prod.yml up -d --scale the_monkeys_gateway=2
docker-compose -f docker-compose.prod.yml up -d --scale the_monkeys_user=2
```

### 4. Production Health Checks
```bash
# Monitor health status
watch 'docker ps --format "table {{.Names}}\\t{{.Status}}\\t{{.Ports}}"'

# Check specific service health
docker exec the_monkeys_authz grpc_health_probe -addr=:50051
curl http://localhost:8081/healthz  # Gateway health
curl http://localhost:51057/health  # AI Engine health
```

### Production Environment Features
- **Resource limits** for all containers
- **Restart policies** for automatic recovery
- **Security hardening** with non-root users
- **JSON logging** for centralized collection
- **Performance optimization** with sampling

## 🔍 Health Monitoring

### Built-in Health Checks

#### gRPC Health Checks (Go Services)
All Go microservices include gRPC health servers:

```bash
# Check authz service (port 50051)
docker exec the_monkeys_authz grpc_health_probe -addr=:50051

# Check blog service (port 50052)
docker exec the_monkeys_blog grpc_health_probe -addr=:50052

# Check user service (port 50053)
docker exec the_monkeys_user grpc_health_probe -addr=:50053

# Check storage service (port 50054)
docker exec the_monkeys_storage grpc_health_probe -addr=:50054

# Check notification service (port 50055)
docker exec the_monkeys_notification grpc_health_probe -addr=:50055
```

#### HTTP Health Checks
```bash
# Gateway service health
curl http://localhost:8081/healthz

# AI Engine health (detailed status)
curl http://localhost:51057/health
```

### Container Health Status
```bash
# View all container health
docker ps --format "table {{.Names}}\\t{{.Status}}"

# Monitor continuously
watch 'docker ps --format "table {{.Names}}\\t{{.Status}}"'
```

### Logging and Monitoring
```bash
# View aggregated logs
docker-compose logs -f

# Service-specific logs
docker-compose logs -f the_monkeys_authz
docker-compose logs -f the_monkeys_gateway

# Real-time log monitoring
docker-compose logs -f --tail=50
```

## 🔧 Troubleshooting

### Common Issues

#### 1. Port Conflicts
```bash
# Check port usage
netstat -an | grep :8080
netstat -an | grep :5432

# Kill conflicting processes
sudo lsof -ti:8080 | xargs kill -9
```

#### 2. Container Build Failures
```bash
# Clear Docker cache
docker system prune -a

# Rebuild without cache
docker-compose build --no-cache

# Check available disk space
df -h
```

#### 3. Database Connection Issues
```bash
# Check database status
docker exec the_monkeys_db pg_isready

# Connect to database
docker exec -it the_monkeys_db psql -U root -d the_monkeys_user_dev

# Reset database
docker-compose down -v
docker-compose up -d the_monkeys_db
```

#### 4. Service Communication Failures
```bash
# Check network connectivity
docker network ls
docker network inspect the_monkeys_engine_default

# Test service communication
docker exec the_monkeys_gateway ping the_monkeys_authz
docker exec the_monkeys_authz nslookup the_monkeys_db
```

### Health Check Debugging
```bash
# Check gRPC health service registration
docker exec the_monkeys_authz netstat -tlnp | grep :50051

# Verify health probe binary
docker exec the_monkeys_authz which grpc_health_probe

# Test health endpoint manually
docker exec the_monkeys_authz grpc_health_probe -addr=localhost:50051 -service=AuthzService
```

### Log Analysis
```bash
# Search for errors
docker-compose logs | grep -i error

# Filter by service and time
docker-compose logs --since="1h" the_monkeys_authz | grep -i warn

# Export logs for analysis
docker-compose logs --no-color > deployment-logs.txt
```

## 📈 Scaling & Optimization

### Horizontal Scaling
```bash
# Scale services based on load
docker-compose up -d --scale the_monkeys_authz=3
docker-compose up -d --scale the_monkeys_user=2
docker-compose up -d --scale the_monkeys_gateway=2
```

### Resource Monitoring
```bash
# Monitor resource usage
docker stats

# Detailed container metrics
docker exec the_monkeys_authz top
docker exec the_monkeys_authz free -m
```

### Performance Optimization
```bash
# Enable production logging with sampling
# In .env file:
LOG_LEVEL=info
LOG_FORMAT=json
LOG_SAMPLING=1

# Use distroless containers for production
docker-compose -f docker-compose.prod.yml up -d
```

### Container Size Optimization
Our optimized containers achieve:
- **authz**: 587MB → 56.2MB (96% reduction)
- **blog**: 662MB → 57.1MB (91% reduction)
- **user**: 662MB → 56.8MB (91% reduction)
- **storage**: 662MB → 57.0MB (91% reduction)
- **notification**: 662MB → 56.9MB (91% reduction)
- **gateway**: 662MB → 57.2MB (91% reduction)
- **AI engine**: 1.2GB → 180MB (85% reduction)

## 🔄 Maintenance

### Regular Maintenance Tasks
```bash
# Update container images
docker-compose pull
docker-compose up -d

# Clean up unused resources
docker system prune -f

# Backup database
docker exec the_monkeys_db pg_dump -U root the_monkeys_user_dev > backup.sql

# Rotate logs
docker-compose logs --no-color > logs/deployment-$(date +%Y%m%d).log
```

### Version Updates
```bash
# Update to latest version
git pull origin main
docker-compose build --no-cache
docker-compose up -d
```

---

## 📞 Support

For deployment issues:
1. Check the [Troubleshooting](#troubleshooting) section
2. Review logs with `docker-compose logs`
3. Verify environment configuration in `.env`
4. Check [QUICK_DEPLOY.md](QUICK_DEPLOY.md) for common solutions

**Next Steps**: See [DEPLOYMENT_SUMMARY.md](DEPLOYMENT_SUMMARY.md) for optimization results and [CONTAINER_BUILD_PROCESS.md](CONTAINER_BUILD_PROCESS.md) for technical details.