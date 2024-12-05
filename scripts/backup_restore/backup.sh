#!/bin/bash

set -x

# Variables
DATABASE="<database name>"
BACKUP_DIR="/var/lib/postgresql/backup"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="${BACKUP_DIR}/${DATABASE}_${TIMESTAMP}.bak"
REMOTE_USER="<Host Username>"
REMOTE_HOST="<Host IP>"
REMOTE_DIR="/path/name"

# Ensure backup directory exists
sudo -u postgres mkdir -p "$BACKUP_DIR"

# Run pg_dump as the postgres user
sudo -u postgres pg_dump -d "$DATABASE" -F c -b -v -f "$BACKUP_FILE"

# Check if pg_dump was successful
if [ $? -eq 0 ]; then
    echo "Backup successful: $BACKUP_FILE"

    # Copy the backup file to the remote machine
    scp "$BACKUP_FILE" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR"

    # Check if scp was successful
    if [ $? -eq 0 ]; then
        echo "Backup file copied to $REMOTE_HOST:$REMOTE_DIR"
    else
        echo "Error copying backup file to remote host."
        exit 1
    fi
else
    echo "Backup failed!"
    exit 1
fi
