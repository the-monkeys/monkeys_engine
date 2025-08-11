#!/usr/bin/env pwsh

# PowerShell script to restore PostgreSQL backup from backup

param(
    [string]$ContainerName = "the-monkeys-psql",
    [string]$DatabaseName = "the_monkeys_user_dev",
    [string]$Username = "root",
    [string]$DatabasePassword = "Secret",  # Note: In production, use SecureString or PSCredential
    [string]$BackupDir = "./postgres_backup"
)

# Colors for output
$Red = "`e[0;31m"
$Green = "`e[0;32m"
$Yellow = "`e[1;33m"
$Cyan = "`e[0;36m"
$White = "`e[1;37m"
$Gray = "`e[0;37m"
$NC = "`e[0m" # No Color

Write-Host "${Green}Starting PostgreSQL backup restoration process...${NC}"

# Check if Docker is running
Write-Host "${Yellow}Checking if Docker is running...${NC}"
try {
    $dockerVersion = docker version --format '{{.Server.Version}}' 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Docker not accessible"
    }
    Write-Host "${Green}Docker is running (version: $dockerVersion)${NC}"
}
catch {
    Write-Host "${Red}Error: Docker is not running or not accessible${NC}"
    Write-Host "${Red}Please start Docker and try again${NC}"
    exit 1
}

# Check if PostgreSQL container is running
Write-Host "${Yellow}Checking if PostgreSQL container is running...${NC}"
$containerStatus = docker ps --filter "name=$ContainerName" --format "{{.Status}}" 2>$null

if (-not $containerStatus) {
    Write-Host "${Red}Error: PostgreSQL container '$ContainerName' is not running${NC}"
    Write-Host "${Yellow}Starting the container...${NC}"
    docker start $ContainerName
    if ($LASTEXITCODE -ne 0) {
        Write-Host "${Red}Failed to start PostgreSQL container${NC}"
        exit 1
    }
}

# Wait for PostgreSQL to be ready
Write-Host "${Yellow}Waiting for PostgreSQL to be ready...${NC}"
$maxAttempts = 30
$attempt = 0

while ($attempt -lt $maxAttempts) {
    $attempt++
    docker exec -e PGPASSWORD=$DatabasePassword $ContainerName pg_isready -U $Username 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "${Green}PostgreSQL is ready!${NC}"
        break
    }
    else {
        Write-Host "${Gray}   PostgreSQL not ready yet, waiting 5 seconds... (attempt $attempt/$maxAttempts)${NC}"
        Start-Sleep -Seconds 5
    }
}

if ($attempt -eq $maxAttempts) {
    Write-Host "${Red}Error: PostgreSQL failed to become ready after $maxAttempts attempts${NC}"
    exit 1
}

# Find the latest backup file
Write-Host "${Cyan}Looking for the latest backup file...${NC}"
if (-not (Test-Path $BackupDir)) {
    Write-Host "${Red}Error: Backup directory '$BackupDir' does not exist${NC}"
    exit 1
}

$backupFiles = Get-ChildItem -Path $BackupDir -Filter "*.bak" | Sort-Object LastWriteTime -Descending
if ($backupFiles.Count -eq 0) {
    Write-Host "${Red}Error: No backup files (*.bak) found in '$BackupDir'${NC}"
    exit 1
}

$latestBackup = $backupFiles[0]
Write-Host "${Green}Latest backup found: $($latestBackup.Name)${NC}"
Write-Host "${Gray}   File size: $([math]::Round($latestBackup.Length / 1MB, 2)) MB${NC}"
Write-Host "${Gray}   Created: $($latestBackup.LastWriteTime)${NC}"

# Confirm restoration
Write-Host "${Yellow}This will restore the database '$DatabaseName' from backup: $($latestBackup.Name)${NC}"
Write-Host "${Yellow}WARNING: This will overwrite all existing data in the database!${NC}"
$confirmation = Read-Host "Do you want to continue? (y/N)"

if ($confirmation -ne 'y' -and $confirmation -ne 'Y') {
    Write-Host "${Yellow}Restoration cancelled by user${NC}"
    exit 0
}

# Create a temporary database for restoration
$tempDbName = "${DatabaseName}_restore_temp_$(Get-Date -Format 'yyyyMMdd_HHmmss')"
Write-Host "${Cyan}Creating temporary database '$tempDbName'...${NC}"

docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d postgres -c "CREATE DATABASE `"$tempDbName`";" 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "${Red}Failed to create temporary database${NC}"
    exit 1
}

# Restore backup to temporary database
Write-Host "${Cyan}Restoring backup to temporary database...${NC}"
$backupPath = "/backup_source/$($latestBackup.Name)"

# Check if backup file exists in container
docker exec $ContainerName test -f $backupPath 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "${Red}Error: Backup file not found in container at path: $backupPath${NC}"
    # Clean up temporary database
    docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d postgres -c "DROP DATABASE `"$tempDbName`";" 2>$null
    exit 1
}

# Restore the backup
Write-Host "${Yellow}Restoring backup... This may take several minutes...${NC}"
$restoreOutput = docker exec -e PGPASSWORD=$DatabasePassword $ContainerName pg_restore -U $Username -d $tempDbName --no-owner --verbose /backup_source/$($latestBackup.Name) 2>&1

# Check if restore had critical errors (not just warnings)
$criticalErrors = $restoreOutput | Where-Object { $_ -match "error:" -and $_ -notmatch "role.*does not exist" -and $_ -notmatch "DEFAULT PRIVILEGES" }

if ($LASTEXITCODE -ne 0 -and $criticalErrors.Count -gt 0) {
    Write-Host "${Red}Failed to restore backup to temporary database${NC}"
    Write-Host "${Red}Critical errors found:${NC}"
    $criticalErrors | ForEach-Object { Write-Host "${Red}  $_${NC}" }
    # Clean up temporary database
    docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d postgres -c "DROP DATABASE `"$tempDbName`";" 2>$null
    exit 1
}
elseif ($restoreOutput | Where-Object { $_ -match "warning:" }) {
    $warningCount = ($restoreOutput | Where-Object { $_ -match "warning:" }).Count
    Write-Host "${Yellow}Restore completed with $warningCount warnings (mostly about role ownership - this is expected)${NC}"
}

Write-Host "${Green}Backup restored successfully to temporary database!${NC}"

# Terminate connections to the target database
Write-Host "${Cyan}Terminating connections to database '$DatabaseName'...${NC}"
$terminateConnections = @"
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = '$DatabaseName' AND pid <> pg_backend_pid();
"@

docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d postgres -c $terminateConnections 2>$null

# Drop the old database and rename the temporary one
Write-Host "${Cyan}Replacing old database with restored data...${NC}"

docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d postgres -c "DROP DATABASE IF EXISTS `"$DatabaseName`";" 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "${Red}Failed to drop old database${NC}"
    # Clean up temporary database
    docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d postgres -c "DROP DATABASE `"$tempDbName`";" 2>$null
    exit 1
}

docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d postgres -c "ALTER DATABASE `"$tempDbName`" RENAME TO `"$DatabaseName`";" 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "${Red}Failed to rename temporary database${NC}"
    exit 1
}

# Verify restoration
Write-Host "${Cyan}Verifying database restoration...${NC}"
$tableCount = docker exec -e PGPASSWORD=$DatabasePassword $ContainerName psql -U $Username -d $DatabaseName -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>$null

if ($LASTEXITCODE -eq 0) {
    $tableCount = $tableCount.Trim()
    Write-Host "${Green}Database restoration completed successfully!${NC}"
    Write-Host "${White}Database: $DatabaseName${NC}"
    Write-Host "${White}Tables restored: $tableCount${NC}"
    Write-Host "${White}Backup file: $($latestBackup.Name)${NC}"
}
else {
    Write-Host "${Yellow}Warning: Could not verify table count, but restoration appears successful${NC}"
}

# Show database info
Write-Host "${Cyan}Database connection information:${NC}"
Write-Host "${White}  Host: localhost${NC}"
Write-Host "${White}  Port: 1234${NC}"
Write-Host "${White}  Database: $DatabaseName${NC}"
Write-Host "${White}  Username: $Username${NC}"

Write-Host "${Green}PostgreSQL restoration process completed!${NC}"
Write-Host "${Green}Your database should now be restored from the latest backup!${NC}"
