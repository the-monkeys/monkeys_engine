# Database Restore Scripts

This repository contains scripts to restore database backups for The Monkeys Engine project.

## Avai3. **"Failed to restore"**
   - Check container logs: `docker logs <container-name>`
   - Ensure sufficient disk space
   - Verify backup file integrity
   - **Note**: Some warnings about missing roles (e.g., "role 'monkeys_engine' does not exist") are normal and expected when restoring production backups to development environmentse Scripts

### 1. Elasticsearch Snapshot Restore
- **File**: `restore-snapshots.sh`
- **Platform**: Linux/macOS (Bash)
- **Purpose**: Restores the latest Elasticsearch snapshot from the backup repository

### 2. PostgreSQL Backup Restore
- **Files**: 
  - `restore-postgres.ps1` (Windows PowerShell)
  - `restore-postgres.sh` (Linux/macOS Bash)
- **Purpose**: Restores the latest PostgreSQL backup from the backup directory

## Prerequisites

1. **Docker** must be running
2. **Docker Compose** services should be up (or at least the database containers)
3. Backup files must be present in their respective directories:
   - Elasticsearch: `./elasticsearch_snapshots/`
   - PostgreSQL: `./postgres_backup/`

## Usage

### Elasticsearch Restore

```bash
# Make sure Docker services are running
docker-compose up -d elasticsearch-node1

# Run the restore script
./restore-snapshots.sh
```

### PostgreSQL Restore

#### On Windows (PowerShell)
```powershell
# Make sure Docker services are running
docker-compose up -d the_monkeys_db

# Run the restore script
.\restore-postgres.ps1
```

#### On Linux/macOS (Bash)
```bash
# Make sure Docker services are running
docker-compose up -d the_monkeys_db

# Make script executable (first time only)
chmod +x restore-postgres.sh

# Run the restore script
./restore-postgres.sh
```

## Script Features

### Common Features (Both Scripts)
- ✅ **Automatic Detection**: Finds the latest backup file automatically
- ✅ **Health Checks**: Waits for database services to be ready before proceeding
- ✅ **Colored Output**: User-friendly colored terminal output
- ✅ **Error Handling**: Comprehensive error checking and cleanup
- ✅ **Confirmation**: Asks for user confirmation before destructive operations

### PostgreSQL Script Specific Features
- ✅ **Safe Restoration**: Uses temporary database to avoid data loss during restore
- ✅ **Atomic Operation**: Database swap happens atomically
- ✅ **Connection Termination**: Safely terminates existing connections before restore
- ✅ **Verification**: Verifies restoration success by checking table count
- ✅ **Cleanup**: Automatic cleanup of temporary resources on failure
- ✅ **Ownership Handling**: Uses `--no-owner` flag to handle production backups with different owners

### Elasticsearch Script Specific Features
- ✅ **Repository Registration**: Automatically registers snapshot repository
- ✅ **Latest Snapshot**: Automatically finds and restores the latest snapshot
- ✅ **Index Verification**: Shows restored indices after completion

## Configuration

### PostgreSQL Script Parameters
You can customize the PostgreSQL restore script by modifying these parameters:

```powershell
# PowerShell
.\restore-postgres.ps1 -ContainerName "custom-postgres" -DatabaseName "my_db" -Username "myuser" -BackupDir "./custom_backup"
```

```bash
# Bash - edit variables at the top of the script
CONTAINER_NAME="the-monkeys-psql"
DATABASE_NAME="the_monkeys_user_dev"
USERNAME="root"
PASSWORD="Secret"
BACKUP_DIR="./postgres_backup"
```

## Default Configuration

### PostgreSQL
- **Container Name**: `the-monkeys-psql`
- **Database Name**: `the_monkeys_user_dev`
- **Username**: `root`
- **Password**: `Secret`
- **Port**: `1234`
- **Backup Directory**: `./postgres_backup`
- **Backup Format**: `*.bak` files

### Elasticsearch
- **Container Name**: `elasticsearch-node1`
- **Port**: `9200`
- **Repository Name**: `backup_repo`
- **Snapshot Directory**: `./elasticsearch_snapshots`

## Troubleshooting

### Common Issues

1. **"Docker is not running"**
   - Start Docker Desktop or Docker service
   - Ensure Docker daemon is accessible

2. **"Container not running"**
   - Start the required containers: `docker-compose up -d <service>`

3. **"No backup files found"**
   - Verify backup files exist in the correct directory
   - Check file permissions and formats

4. **"Failed to restore"**
   - Check container logs: `docker logs <container-name>`
   - Ensure sufficient disk space
   - Verify backup file integrity

### Log Monitoring
Monitor the restoration process:
```bash
# PostgreSQL logs
docker logs -f the-monkeys-psql

# Elasticsearch logs
docker logs -f elasticsearch-node1
```

## Security Notes

- The scripts contain hardcoded passwords for development environments
- For production use, consider using:
  - Environment variables
  - Docker secrets
  - External credential management systems

## File Locations

- **Elasticsearch Snapshots**: `./elasticsearch_snapshots/`
- **PostgreSQL Backups**: `./postgres_backup/`
- **Docker Volumes**: Managed by Docker Compose

## Support

If you encounter issues:
1. Check the container logs
2. Verify backup file integrity
3. Ensure Docker services are healthy
4. Check disk space availability
