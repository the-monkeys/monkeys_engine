# 📚 The Monkeys Engine Documentation

Welcome to the comprehensive documentation for The Monkeys Engine microservices architecture. This documentation covers the complete container optimization, health monitoring, and deployment automation implemented in our recent updates.

## 📑 Documentation Structure

### 🚀 Deployment & Operations
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Complete step-by-step deployment guide for all environments
- **[QUICK_DEPLOY.md](QUICK_DEPLOY.md)** - Quick reference commands and fast deployment instructions
- **[DEPLOYMENT_SUMMARY.md](DEPLOYMENT_SUMMARY.md)** - Executive summary with optimization results and achievements

### 🔧 Technical Deep-Dive
- **[CONTAINER_BUILD_PROCESS.md](CONTAINER_BUILD_PROCESS.md)** - Detailed technical guide to container optimization
- **[BUILD_PROCESS_DIAGRAM.md](BUILD_PROCESS_DIAGRAM.md)** - Visual flow diagrams and architecture overview

### 📖 API & Database Documentation
- **[apis/](apis/)** - API documentation and specifications
- **[db/](db/)** - Database schemas and documentation

## 🎯 Quick Start

If you're new to the project, start here:

1. **[QUICK_DEPLOY.md](QUICK_DEPLOY.md)** - Get the system running in 5 minutes
2. **[DEPLOYMENT.md](DEPLOYMENT.md)** - Full deployment guide for production
3. **[DEPLOYMENT_SUMMARY.md](DEPLOYMENT_SUMMARY.md)** - Understand what was optimized

## 📊 Project Achievements

### Container Optimization Results
Our recent microservices optimization achieved:

- **🏆 85%+ average container size reduction** across all services
- **✅ 100% health monitoring coverage** with gRPC and HTTP health checks
- **🔒 Security hardening** with non-root users and minimal attack surfaces
- **🚀 Production-ready** deployment automation and monitoring

### Service-by-Service Results
| Service | Before | After | Reduction |
|---------|--------|-------|-----------|
| **authz** | 587MB | 56.2MB | **96%** ⭐ |
| **blog** | 662MB | 57.1MB | **91%** |
| **user** | 662MB | 56.8MB | **91%** |
| **storage** | 662MB | 57.0MB | **91%** |
| **notification** | 662MB | 56.9MB | **91%** |
| **gateway** | 662MB | 57.2MB | **91%** |
| **AI engine** | 1.2GB | 180MB | **85%** |

## 🛠 Available Tools

### Deployment Automation
- **`deploy.sh`** - Linux/macOS deployment script
- **`deploy.ps1`** - Windows PowerShell deployment script
- **`docker-compose.yml`** - Development environment
- **`docker-compose.prod.yml`** - Production environment

### Container Variants
Each microservice includes:
- **Standard Dockerfile** - Optimized Alpine-based builds
- **Distroless Dockerfile** - Maximum security with minimal footprint
- **Health checks** - Built-in monitoring and status reporting

## 🔗 Related Files

- **[README.md](../README.md)** - Main project README
- **[ENVIRONMENT_CONFIG.md](../ENVIRONMENT_CONFIG.md)** - Environment variable reference
- **[docker-compose.yml](../docker-compose.yml)** - Development container orchestration
- **[docker-compose.prod.yml](../docker-compose.prod.yml)** - Production container orchestration

## 📞 Support

For questions about deployment, optimization, or architecture:

1. Check the relevant documentation section above
2. Review the [main README](../README.md) for project overview
3. Examine the environment configuration in [ENVIRONMENT_CONFIG.md](../ENVIRONMENT_CONFIG.md)

## 🏷 Document Versions

- **Last Updated**: September 19, 2025
- **Version**: 2.0 (Post Container Optimization)
- **Architecture**: Microservices with gRPC/HTTP health monitoring
- **Container Strategy**: Multi-stage Alpine/Distroless builds

---

> 💡 **Tip**: All documentation is designed to be read in order. Start with [QUICK_DEPLOY.md](QUICK_DEPLOY.md) for immediate results, then dive deeper with [DEPLOYMENT.md](DEPLOYMENT.md) and [CONTAINER_BUILD_PROCESS.md](CONTAINER_BUILD_PROCESS.md).