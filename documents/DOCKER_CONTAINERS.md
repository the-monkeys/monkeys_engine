## 🐳 Automated Docker Containers

This project automatically builds and publishes Docker containers to GitHub Container Registry on every push to `main`.

### 📦 Available Images

All microservices are automatically built and available at:
```
ghcr.io/the-monkeys/monkeys-gateway:latest
ghcr.io/the-monkeys/monkeys-ai-engine:latest
ghcr.io/the-monkeys/monkeys-blog:latest
ghcr.io/the-monkeys/monkeys-auth:latest
ghcr.io/the-monkeys/monkeys-user:latest
ghcr.io/the-monkeys/monkeys-notification:latest
ghcr.io/the-monkeys/monkeys-storage:latest
```

### 🚀 Quick Deploy

1. **Download the deployment file:**
   ```bash
   curl -O https://raw.githubusercontent.com/the-monkeys/the_monkeys_engine/main/.env.example
   mv .env.example .env
   ```

2. **Configure your environment:**
   ```bash
   nano .env  # Edit with your settings
   ```

3. **Deploy with pre-built containers:**
   ```bash
   # Get the registry compose file (when available)
   curl -O https://github.com/the-monkeys/the_monkeys_engine/releases/latest/download/docker-compose.registry.yml
   docker-compose -f docker-compose.registry.yml up -d
   ```

### 🔄 How It Works

- **Every push to `main`** → Automatic container build
- **No manual setup required** → Everything is automated
- **Multi-platform support** → AMD64 and ARM64
- **Semantic versioning** → Tag releases for version-specific deploys
- **Zero credentials needed** → Safe for opensource projects

### 📊 Build Status

Check the latest build status: [GitHub Actions](https://github.com/the-monkeys/the_monkeys_engine/actions)

View available containers: [GitHub Packages](https://github.com/orgs/the-monkeys/packages?repo_name=the_monkeys_engine)

---

For detailed deployment instructions, see [GHCR_DEPLOYMENT.md](./GHCR_DEPLOYMENT.md)