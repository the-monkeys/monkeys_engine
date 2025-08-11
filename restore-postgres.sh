#!/bin/bash

# Bash script to restore PostgreSQL backup from backup

set -e

# Configuration
CONTAINER_NAME="the-monkeys-psql"
DATABASE_NAME="the_monkeys_user_dev"
USERNAME="root"
PASSWORD="Secret"
BACKUP_DIR="./postgres_backup"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
GRAY='\033[0;37m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting PostgreSQL backup restoration process...${NC}"

# Check if Docker is running
echo -e "${YELLOW}Checking if Docker is running...${NC}"
if ! docker version > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running or not accessible${NC}"
    echo -e "${RED}Please start Docker and try again${NC}"
    exit 1
fi

DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null)
echo -e "${GREEN}Docker is running (version: $DOCKER_VERSION)${NC}"

# Check if PostgreSQL container is running
echo -e "${YELLOW}Checking if PostgreSQL container is running...${NC}"
if ! docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "$CONTAINER_NAME"; then
    echo -e "${RED}Error: PostgreSQL container '$CONTAINER_NAME' is not running${NC}"
    echo -e "${YELLOW}Starting the container...${NC}"
    if ! docker start "$CONTAINER_NAME"; then
        echo -e "${RED}Failed to start PostgreSQL container${NC}"
        exit 1
    fi
fi

# Wait for PostgreSQL to be ready
echo -e "${YELLOW}Waiting for PostgreSQL to be ready...${NC}"
MAX_ATTEMPTS=30
ATTEMPT=0

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    ATTEMPT=$((ATTEMPT + 1))
    if docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" pg_isready -U "$USERNAME" > /dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    else
        echo -e "${GRAY}   PostgreSQL not ready yet, waiting 5 seconds... (attempt $ATTEMPT/$MAX_ATTEMPTS)${NC}"
        sleep 5
    fi
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    echo -e "${RED}Error: PostgreSQL failed to become ready after $MAX_ATTEMPTS attempts${NC}"
    exit 1
fi

# Find the latest backup file
echo -e "${CYAN}Looking for the latest backup file...${NC}"
if [ ! -d "$BACKUP_DIR" ]; then
    echo -e "${RED}Error: Backup directory '$BACKUP_DIR' does not exist${NC}"
    exit 1
fi

LATEST_BACKUP=$(find "$BACKUP_DIR" -name "*.bak" -type f -printf '%T@ %p\n' 2>/dev/null | sort -n | tail -1 | cut -d' ' -f2-)

if [ -z "$LATEST_BACKUP" ]; then
    echo -e "${RED}Error: No backup files (*.bak) found in '$BACKUP_DIR'${NC}"
    exit 1
fi

BACKUP_NAME=$(basename "$LATEST_BACKUP")
BACKUP_SIZE=$(du -h "$LATEST_BACKUP" | cut -f1)
BACKUP_DATE=$(stat -c %y "$LATEST_BACKUP" 2>/dev/null || stat -f %Sm "$LATEST_BACKUP" 2>/dev/null)

echo -e "${GREEN}Latest backup found: $BACKUP_NAME${NC}"
echo -e "${GRAY}   File size: $BACKUP_SIZE${NC}"
echo -e "${GRAY}   Created: $BACKUP_DATE${NC}"

# Confirm restoration
echo -e "${YELLOW}This will restore the database '$DATABASE_NAME' from backup: $BACKUP_NAME${NC}"
echo -e "${YELLOW}WARNING: This will overwrite all existing data in the database!${NC}"
read -p "Do you want to continue? (y/N): " confirmation

if [ "$confirmation" != "y" ] && [ "$confirmation" != "Y" ]; then
    echo -e "${YELLOW}Restoration cancelled by user${NC}"
    exit 0
fi

# Create a temporary database for restoration
TEMP_DB_NAME="${DATABASE_NAME}_restore_temp_$(date +%Y%m%d_%H%M%S)"
echo -e "${CYAN}Creating temporary database '$TEMP_DB_NAME'...${NC}"

if ! docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d postgres -c "CREATE DATABASE \"$TEMP_DB_NAME\";" > /dev/null 2>&1; then
    echo -e "${RED}Failed to create temporary database${NC}"
    exit 1
fi

# Restore backup to temporary database
echo -e "${CYAN}Restoring backup to temporary database...${NC}"
BACKUP_PATH="/backup_source/$BACKUP_NAME"

# Check if backup file exists in container
if ! docker exec "$CONTAINER_NAME" test -f "$BACKUP_PATH" > /dev/null 2>&1; then
    echo -e "${RED}Error: Backup file not found in container at path: $BACKUP_PATH${NC}"
    # Clean up temporary database
    docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d postgres -c "DROP DATABASE \"$TEMP_DB_NAME\";" > /dev/null 2>&1
    exit 1
fi

# Restore the backup
echo -e "${YELLOW}Restoring backup... This may take several minutes...${NC}"
RESTORE_OUTPUT=$(docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" pg_restore -U "$USERNAME" -d "$TEMP_DB_NAME" --no-owner --verbose "$BACKUP_PATH" 2>&1)
RESTORE_EXIT_CODE=$?

# Check if restore had critical errors (not just warnings)
CRITICAL_ERRORS=$(echo "$RESTORE_OUTPUT" | grep "error:" | grep -v "role.*does not exist" | grep -v "DEFAULT PRIVILEGES")

if [ $RESTORE_EXIT_CODE -ne 0 ] && [ -n "$CRITICAL_ERRORS" ]; then
    echo -e "${RED}Failed to restore backup to temporary database${NC}"
    echo -e "${RED}Critical errors found:${NC}"
    echo "$CRITICAL_ERRORS" | while read -r error; do
        echo -e "${RED}  $error${NC}"
    done
    # Clean up temporary database
    docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d postgres -c "DROP DATABASE \"$TEMP_DB_NAME\";" > /dev/null 2>&1
    exit 1
elif echo "$RESTORE_OUTPUT" | grep -q "warning:"; then
    WARNING_COUNT=$(echo "$RESTORE_OUTPUT" | grep -c "warning:")
    echo -e "${YELLOW}Restore completed with $WARNING_COUNT warnings (mostly about role ownership - this is expected)${NC}"
fi

echo -e "${GREEN}Backup restored successfully to temporary database!${NC}"

# Terminate connections to the target database
echo -e "${CYAN}Terminating connections to database '$DATABASE_NAME'...${NC}"
TERMINATE_QUERY="SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DATABASE_NAME' AND pid <> pg_backend_pid();"
docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d postgres -c "$TERMINATE_QUERY" > /dev/null 2>&1

# Drop the old database and rename the temporary one
echo -e "${CYAN}Replacing old database with restored data...${NC}"

if ! docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d postgres -c "DROP DATABASE IF EXISTS \"$DATABASE_NAME\";" > /dev/null 2>&1; then
    echo -e "${RED}Failed to drop old database${NC}"
    # Clean up temporary database
    docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d postgres -c "DROP DATABASE \"$TEMP_DB_NAME\";" > /dev/null 2>&1
    exit 1
fi

if ! docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d postgres -c "ALTER DATABASE \"$TEMP_DB_NAME\" RENAME TO \"$DATABASE_NAME\";" > /dev/null 2>&1; then
    echo -e "${RED}Failed to rename temporary database${NC}"
    exit 1
fi

# Verify restoration
echo -e "${CYAN}Verifying database restoration...${NC}"
TABLE_COUNT=$(docker exec -e PGPASSWORD="$PASSWORD" "$CONTAINER_NAME" psql -U "$USERNAME" -d "$DATABASE_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | tr -d ' ')

if [ $? -eq 0 ] && [ -n "$TABLE_COUNT" ]; then
    echo -e "${GREEN}Database restoration completed successfully!${NC}"
    echo -e "${WHITE}Database: $DATABASE_NAME${NC}"
    echo -e "${WHITE}Tables restored: $TABLE_COUNT${NC}"
    echo -e "${WHITE}Backup file: $BACKUP_NAME${NC}"
else
    echo -e "${YELLOW}Warning: Could not verify table count, but restoration appears successful${NC}"
fi

# Show database info
echo -e "${CYAN}Database connection information:${NC}"
echo -e "${WHITE}  Host: localhost${NC}"
echo -e "${WHITE}  Port: 1234${NC}"
echo -e "${WHITE}  Database: $DATABASE_NAME${NC}"
echo -e "${WHITE}  Username: $USERNAME${NC}"

echo -e "${GREEN}PostgreSQL restoration process completed!${NC}"
echo -e "${GREEN}Your database should now be restored from the latest backup!${NC}"
