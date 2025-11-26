# MinIO Data Restore Guide

## Quick Restore

Restore all data from remote backup to local MinIO in one command:

```bash
docker compose --profile restore up minio-restore
```

## What It Does

1. Connects to remote MinIO server at `storage_url:9000`
2. Syncs all data from remote bucket to local MinIO
3. Exits automatically when complete

## When to Use

- **Disaster recovery**: Lost local MinIO data/volume
- **Fresh setup**: New environment needs existing data
- **Data corruption**: Need to restore clean backup

## Configuration

All settings in `.env`:
```env
MINIO_REMOTE_ENDPOINT=http://storage_url:9000
MINIO_REMOTE_ACCESS_KEY=your_access_key
MINIO_REMOTE_SECRET_KEY=your_secret_key
MINIO_REMOTE_BUCKET_NAME=themonkeys-storage
```

## Normal Operations

- **Automatic sync**: `minio-sync` container continuously backs up local → remote
- **Manual restore**: Run restore command only when needed (uses Docker profile)
- **No conflicts**: Restore and sync run independently

## Complete Recovery Example

```bash
# 1. Stop and remove everything (including volumes)
docker compose down -v

# 2. Start fresh
docker compose up -d

# 3. Restore data
docker compose --profile restore up minio-restore
```

That's it! Your data is recovered.
