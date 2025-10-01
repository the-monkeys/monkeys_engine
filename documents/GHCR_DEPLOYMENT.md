# 🐒 The Monkeys Engine - Automated Docker Container Registry

This project automatically builds and publishes Docker containers to GitHub Container Registry (ghcr.io) on every push to the main branch. No manual setup required!

## 🚀 Automatic Container Builds

### 🔄 How It Works

Every time you push to the `main` branch, GitHub Actions automatically:
- ✅ **Builds all 7 microservices** using multi-stage Dockerfiles
- ✅ **Pushes to GitHub Container Registry** at `ghcr.io/the-monkeys/monkeys-*`
- ✅ **Supports multi-platform** (AMD64 + ARM64)
- ✅ **Tags appropriately** (latest, branch name, commit SHA, semver)
- ✅ **Generates deployment files** for easy deployment

### 📦 No Setup Required

As an **opensource project**, everything is automated:
- **No tokens needed** - Uses GitHub's built-in `GITHUB_TOKEN`
- **No manual builds** - Triggered automatically on push
- **No credentials** - All configuration is public and safe
- **No secrets exposed** - Environment variables are template-only

## 📦 Available Container Images

After any push to `main`, images are automatically available at:
- `ghcr.io/the-monkeys/monkeys-gateway:latest`
- `ghcr.io/the-monkeys/monkeys-ai-engine:latest` 
- `ghcr.io/the-monkeys/monkeys-blog:latest`
- `ghcr.io/the-monkeys/monkeys-auth:latest`
- `ghcr.io/the-monkeys/monkeys-user:latest`
- `ghcr.io/the-monkeys/monkeys-notification:latest`
- `ghcr.io/the-monkeys/monkeys-storage:latest`

### �️ Available Tags
- `latest` - Latest main branch build
- `main` - Main branch builds  
- `<commit-sha>` - Specific commit builds
- `v1.2.3` - Semantic version releases (when tagged)

## 🎯 Easy Deployment

### Option 1: Download Registry Compose (Recommended)

```bash
# Download the pre-configured compose file from GitHub releases
curl -O https://github.com/the-monkeys/the_monkeys_engine/releases/latest/download/docker-compose.registry.yml

# Configure your environment
cp .env.example .env
# Edit .env with your settings

# Deploy using pre-built images
docker-compose -f docker-compose.registry.yml up -d
```

### Option 2: Pull and Run Individual Services

```bash
# Pull any specific image
docker pull ghcr.io/the-monkeys/monkeys-gateway:latest
docker pull ghcr.io/the-monkeys/monkeys-ai-engine:latest

# Run with your custom configuration
docker run -d --name gateway ghcr.io/the-monkeys/monkeys-gateway:latest
```

### Option 3: Deploy to Production Server

```bash
# On your production server:
# 1. Download the registry compose
wget https://raw.githubusercontent.com/the-monkeys/the_monkeys_engine/main/.env.example -O .env

# 2. Configure environment variables
nano .env  # Edit with your production values

# 3. Deploy with pre-built images
curl -O https://github.com/the-monkeys/the_monkeys_engine/releases/latest/download/docker-compose.registry.yml
docker-compose -f docker-compose.registry.yml up -d
```

## ⚙️ Configuration

### 🔧 Automated Build Configuration

The GitHub Actions workflow (`.github/workflows/build-and-push.yml`) automatically:

- **Triggers on**: 
  - Push to `main` branch
  - Pull requests to `main` 
  - Manual workflow dispatch
  - GitHub releases
- **Build Features**: 
  - Multi-stage Docker builds for optimal size
  - Multi-platform support (AMD64 + ARM64)
  - Optimized layer caching for fast builds
  - Automatic build args injection
- **Security**: 
  - Uses GitHub's built-in `GITHUB_TOKEN`
  - No external secrets required
  - Safe for opensource projects
- **Tagging Strategy**:
  - `latest` for main branch
  - Branch names for feature branches
  - Commit SHA for specific builds
  - Semantic versioning for releases

## 🔧 Environment Variables

Make sure your `.env` file is properly configured:

```env
# These are used as build arguments for some services
THE_MONKEYS_GATEWAY_HTTP_PORT=8081
THE_MONKEYS_GATEWAY_HTTPS=8443
MICROSERVICES_AI_ENGINE_PORT=50057
MICROSERVICES_AI_ENGINE_HEALTH_PORT=51600

# Database, RabbitMQ, etc. configuration
POSTGRESQL_PRIMARY_DB_DB_PASSWORD=your_password
RABBITMQ_DEFAULT_USER=myuser
RABBITMQ_DEFAULT_PASS=mypassword
# ... etc
```

## 🎪 Working with the Automated System

### For Contributors

```bash
# 1. Make your changes
git add .
git commit -m "Add new feature"

# 2. Push to trigger automatic build
git push origin main
# ↳ GitHub Actions will automatically build and push containers

# 3. Check build status
# Visit: https://github.com/the-monkeys/the_monkeys_engine/actions
```

### For Deployers

```bash
# 1. Always use the latest images (no building required)
docker pull ghcr.io/the-monkeys/monkeys-gateway:latest

# 2. Use the pre-configured compose file
curl -O https://github.com/the-monkeys/the_monkeys_engine/releases/latest/download/docker-compose.registry.yml

# 3. Deploy immediately
docker-compose -f docker-compose.registry.yml up -d
```

### For Releases

```bash
# 1. Create a semantic version tag
git tag v1.2.3
git push origin v1.2.3

# 2. GitHub Actions will automatically:
#    - Build containers with version tags
#    - Create a GitHub release
#    - Attach docker-compose.registry.yml
```

## 🛡️ Security Features

- **Multi-stage builds** for minimal image size
- **Non-root user** (1001:1001) in all containers
- **Health checks** for all services
- **Resource limits** configured
- **Private registry** support with authentication

## 🚀 Benefits of Using GitHub Container Registry

1. **🔒 Private repositories**: Keep your images secure
2. **🌍 Global CDN**: Fast pulls from anywhere
3. **🔄 Version control**: Tag with branches, commits, and semantic versions
4. **📊 Usage analytics**: Track downloads and usage
5. **🤝 Team access**: Fine-grained permissions with GitHub teams
6. **💰 Cost effective**: 500MB free for public repos, generous limits for private

## 🔍 Troubleshooting

### Common Issues

**Authentication Error:**
```bash
# Make sure your token has packages:write permission
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxx"
```

**Build Args Not Applied:**
- Check that build args are properly defined in Dockerfile `ARG` statements
- Verify environment variables are set correctly

**Image Not Found:**
- Ensure the namespace matches your GitHub username
- Check that the image was pushed successfully
- Verify you're logged into the correct registry

**Permission Denied:**
```bash
# Make sure you have write access to the repository
# And your token has the packages:write scope
```

### Logs and Debugging

```bash
# Check GitHub Actions build logs
# Visit: https://github.com/the-monkeys/the_monkeys_engine/actions

# Verify images are available
docker pull ghcr.io/the-monkeys/monkeys-gateway:latest

# Check local images
docker images | grep monkeys

# Test a specific service
docker run --rm ghcr.io/the-monkeys/monkeys-gateway:latest --help
```

### Contributing to Builds

```bash
# Test build locally before pushing (optional)
docker build -f microservices/the_monkeys_gateway/Dockerfile .

# Push changes - automatic build will trigger
git push origin main

# Monitor build progress
# GitHub Actions tab will show real-time progress
```

## 🎉 Zero-Setup Deployment!

With automated container builds, you get:

1. **🚀 Instant deployment** - Just pull and run
2. **🔄 Always up-to-date** - Containers match the latest code  
3. **📦 Multi-platform** - Works on AMD64 and ARM64
4. **🌍 Global availability** - Fast downloads worldwide
5. **🔒 Secure** - No credentials in code, everything automated
6. **📊 Transparent** - All builds are public and auditable

Your Monkeys Engine containers are automatically built and ready for deployment anywhere! 🐒🚀

## 🔗 Quick Links

- **Container Registry**: [GitHub Packages](https://github.com/the-monkeys/the_monkeys_engine/pkgs/container/monkeys-gateway)
- **Build Status**: [GitHub Actions](https://github.com/the-monkeys/the_monkeys_engine/actions)
- **Releases**: [GitHub Releases](https://github.com/the-monkeys/the_monkeys_engine/releases)
- **Source Code**: [Main Repository](https://github.com/the-monkeys/the_monkeys_engine)