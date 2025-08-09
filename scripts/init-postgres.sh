#!/bin/bash

# PostgreSQL initialization script for bitnami/postgresql
# This script runs during container startup to restore data from backup if needed

set -e

echo "=== PostgreSQL Backup Restoration Script ==="
echo "Checking for existing data and backup files..."

# Function to wait for PostgreSQL
wait_for_postgres() {
    echo "Waiting for PostgreSQL to be ready..."
    until PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c '\q' 2>/dev/null; do
        echo "PostgreSQL is unavailable - sleeping"
        sleep 2
    done
    echo "PostgreSQL is ready!"
}

# Function to check if database has data
check_database_data() {
    local table_count
    table_count=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | xargs || echo "0")
    echo "$table_count"
}

# Function to restore from backup
restore_backup() {
    local backup_file="$1"
    echo "Attempting to restore from: $backup_file"
    
    if [[ "$backup_file" == *.bak ]]; then
        echo "Restoring from .bak file (custom format)..."
        PGPASSWORD="$POSTGRES_PASSWORD" pg_restore -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v "$backup_file" 2>&1 || {
            echo "Custom format restore failed, trying as SQL..."
            PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$backup_file" 2>&1 || echo "SQL restore also failed, continuing..."
        }
    elif [[ "$backup_file" == *.sql ]]; then
        echo "Restoring from .sql file..."
        PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$backup_file"
    elif [[ "$backup_file" == *.dump ]]; then
        echo "Restoring from .dump file..."
        PGPASSWORD="$POSTGRES_PASSWORD" pg_restore -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v "$backup_file" 2>&1 || echo "Restore had issues, continuing..."
    else
        echo "Unknown backup file format: $backup_file"
        return 1
    fi
}

# Main execution
echo "Starting backup restoration check..."

# Wait for PostgreSQL to be ready
wait_for_postgres

# Check if database has existing data
TABLE_COUNT=$(check_database_data)
echo "Found $TABLE_COUNT tables in the database"

if [ "$TABLE_COUNT" -eq "0" ]; then
    echo "Database is empty, checking for backup files..."
    
    # Look for backup files in the mounted backup directory
    BACKUP_FILE=$(find /backup_source -name "*.bak" -o -name "*.sql" -o -name "*.dump" 2>/dev/null | head -1)
    
    if [ -n "$BACKUP_FILE" ] && [ -f "$BACKUP_FILE" ]; then
        echo "Found backup file: $BACKUP_FILE"
        restore_backup "$BACKUP_FILE"
        
        # Verify restoration
        NEW_TABLE_COUNT=$(check_database_data)
        echo "After restoration: $NEW_TABLE_COUNT tables found"
        
        if [ "$NEW_TABLE_COUNT" -gt "0" ]; then
            echo "✅ Database restoration completed successfully!"
        else
            echo "⚠️  Restoration completed but no tables detected"
        fi
    else
        echo "No backup files found in /backup_source"
        echo "Available files:"
        ls -la /backup_source/ 2>/dev/null || echo "Backup directory not accessible"
        echo "Starting with empty database"
    fi
else
    echo "✅ Database already contains $TABLE_COUNT tables, skipping restoration"
fi

echo "=== PostgreSQL initialization completed ==="
