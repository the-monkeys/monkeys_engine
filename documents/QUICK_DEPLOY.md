# ⚡ Quick Deploy Guide

Get The Monkeys Engine running in **5 minutes** with these essential commands.

## 🚀 One-Command Deploy

### Development Environment
```bash
# Clone, build, and start everything
git clone https://github.com/the-monkeys/the_monkeys_engine.git
cd the_monkeys_engine
docker-compose up --build -d

# Verify all services are healthy
docker ps
```

### Production Environment
```bash
# Production deployment with optimized containers
docker-compose -f docker-compose.prod.yml up --build -d

# Monitor health status
watch 'docker ps --format "table {{.Names}}\\t{{.Status}}"'
```

## 📋 Essential Commands

### Start/Stop Services
```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Restart specific service
docker-compose restart the_monkeys_authz

# View logs
docker-compose logs -f
```

### Health Checks
```bash
# Check all container status
docker ps

# Test gRPC health (Go services)
docker exec the_monkeys_authz grpc_health_probe -addr=:50051

# Test HTTP health (Gateway & AI)
curl http://localhost:8081/healthz
curl http://localhost:51057/health
```

### Quick Debugging
```bash
# View service logs
docker-compose logs -f the_monkeys_authz

# Connect to database
docker exec -it the_monkeys_db psql -U root -d the_monkeys_user_dev

# Check resource usage
docker stats

# Clean and rebuild
docker-compose down -v
docker system prune -f
docker-compose up --build -d
```

## 🎯 Service Endpoints

| Service | Health Check | Port | Protocol |
|---------|-------------|------|----------|
| **Gateway** | `curl localhost:8081/healthz` | 8081 | HTTP |
| **Authz** | `grpc_health_probe -addr=:50051` | 50051 | gRPC |
| **Blog** | `grpc_health_probe -addr=:50052` | 50052 | gRPC |
| **User** | `grpc_health_probe -addr=:50053` | 50053 | gRPC |
| **Storage** | `grpc_health_probe -addr=:50054` | 50054 | gRPC |
| **Notification** | `grpc_health_probe -addr=:50055` | 50055 | gRPC |
| **AI Engine** | `curl localhost:51057/health` | 51057 | HTTP |

## ⚙️ Environment Setup

### 1. Copy Environment File
```bash
cp .env.example .env
```

### 2. Essential Environment Variables
```properties
# Production settings
APP_ENV=production
LOG_LEVEL=info

# Database
POSTGRESQL_PRIMARY_DB_DB_USERNAME=root
POSTGRESQL_PRIMARY_DB_DB_PASSWORD=YourSecurePassword
POSTGRESQL_PRIMARY_DB_DB_NAME=the_monkeys_user_prod

# Security
JWT_SECRET_KEY=your-256-bit-secret-key
```

### 3. Quick Start Script
```bash
#!/bin/bash
# quick-start.sh

echo "🚀 Starting The Monkeys Engine..."

# Ensure environment file exists
if [ ! -f .env ]; then
    echo "⚠️  Creating .env from template..."
    cp .env.example .env
    echo "✏️  Please edit .env with your configuration"
    exit 1
fi

# Start infrastructure first
echo "🏗️  Starting infrastructure..."
docker-compose up -d the_monkeys_db elasticsearch-node1 rabbitmq the_monkeys_cache minio

# Wait for services to be ready
echo "⏳ Waiting for infrastructure..."
sleep 30

# Start microservices
echo "🔧 Starting microservices..."
docker-compose up -d the_monkeys_authz the_monkeys_blog the_monkeys_user
docker-compose up -d the_monkeys_storage the_monkeys_notification
docker-compose up -d the_monkeys_gateway the_monkeys_ai_engine

# Check health
echo "🔍 Checking service health..."
sleep 15
docker ps --format "table {{.Names}}\\t{{.Status}}"

echo "✅ Deployment complete! Gateway available at: http://localhost:8081"
```

## 🔧 Common Issues & Quick Fixes

### Port Already in Use
```bash
# Find and kill process using port 8081
sudo lsof -ti:8081 | xargs kill -9

# Or use different ports in .env
THE_MONKEYS_GATEWAY_HTTP_PORT=8082
```

### Container Won't Start
```bash
# Check logs for specific service
docker-compose logs the_monkeys_authz

# Rebuild without cache
docker-compose build --no-cache the_monkeys_authz
docker-compose up -d the_monkeys_authz
```

### Database Connection Failed
```bash
# Restart database
docker-compose restart the_monkeys_db

# Check database is ready
docker exec the_monkeys_db pg_isready

# Connect manually to test
docker exec -it the_monkeys_db psql -U root
```

### Out of Memory
```bash
# Clean up Docker resources
docker system prune -a

# Check available memory
free -h

# Reduce container memory limits in docker-compose.yml
```

## 📊 Optimization Results

Our container optimization achieved:

| Service | Before | After | Reduction |
|---------|--------|-------|-----------|
| authz | 587MB | 56.2MB | **96%** |
| blog | 662MB | 57.1MB | **91%** |
| user | 662MB | 56.8MB | **91%** |
| storage | 662MB | 57.0MB | **91%** |
| notification | 662MB | 56.9MB | **91%** |
| gateway | 662MB | 57.2MB | **91%** |
| AI engine | 1.2GB | 180MB | **85%** |

**Total**: 85%+ average size reduction with full health monitoring!

## 🎛️ Deployment Automation

### Windows PowerShell
```powershell
# Use deploy.ps1 (if available)
.\deploy.ps1 -Environment production -Build

# Or manual commands
docker-compose -f docker-compose.prod.yml up --build -d
```

### Linux/macOS Bash
```bash
# Use deploy.sh (if available)
./deploy.sh --env production --build

# Or manual commands
docker-compose -f docker-compose.prod.yml up --build -d
```

## 📞 Need Help?

1. **Check logs**: `docker-compose logs -f [service-name]`
2. **Verify configuration**: Review `.env` file
3. **Resource issues**: Run `docker system prune -f`
4. **Port conflicts**: Use `netstat -tulpn | grep :8081`

**For detailed instructions**, see [DEPLOYMENT.md](DEPLOYMENT.md)

**For technical details**, see [CONTAINER_BUILD_PROCESS.md](CONTAINER_BUILD_PROCESS.md)

---

> 💡 **Pro Tip**: Always check `docker ps` after deployment to ensure all containers show `(healthy)` status!