#!/usr/bin/env bash
set -euo pipefail
set -x

# Ensure jq is installed locally
if ! command -v jq &>/dev/null; then
    echo "jq is required but not installed. Install jq on your local machine (192.168.1.14)."
    exit 1
fi

# Elasticsearch Credentials and URL
ES_USER="elastic"
ES_PASS="ectb=Df=D5cpGpSgEQnW"
ES_URL="http://localhost:9200"   # Adjust if ES is reachable on a different host/port
REPO_NAME="my_backup"

# Remote host information (where the backups currently reside)
REMOTE_HOST="192.168.1.15"
REMOTE_USER="monkeys-es-01"
REMOTE_PASS="SunnyDay2025!"  # Adjust to the actual password of monkeys-es-01 user if needed.
REMOTE_REPO_PATH="/mnt/backups/es-backup"

# Local path to store the pulled backup
LOCAL_REPO_PATH="/mnt/backups"
mkdir -p "${LOCAL_REPO_PATH}"

# 1. Pull the snapshot repository from remote host using SSH and scp
echo "Pulling snapshot repository from ${REMOTE_USER}@${REMOTE_HOST}..."
sshpass -p "${REMOTE_PASS}" scp -r "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_REPO_PATH}" "${LOCAL_REPO_PATH}"
# sshpass -p 'SunnyDay2025!' scp -r monkeys-es-01@192.168.1.15:/mnt/backups/es-backup /mnt/backups

# 2. List out the available snapshots from the local Elasticsearch repository
SNAPSHOTS=$(curl -s -u "${ES_USER}:${ES_PASS}" -X GET "${ES_URL}/_snapshot/${REPO_NAME}/_all" \
          | jq -r '.snapshots | sort_by(.start_time) | .[].snapshot')

echo "Available Snapshots:"
echo "${SNAPSHOTS}"

# Get the latest snapshot
LATEST_SNAPSHOT=$(echo "${SNAPSHOTS}" | tail -n 1)
echo "Latest Snapshot: ${LATEST_SNAPSHOT}"

# 3. Restore the latest snapshot of all indices
echo "Restoring latest snapshot: ${LATEST_SNAPSHOT}"
curl -s -u "${ES_USER}:${ES_PASS}" -X POST "${ES_URL}/_snapshot/${REPO_NAME}/${LATEST_SNAPSHOT}/_restore" \
     -H 'Content-Type: application/json' \
     -d '{
           "indices": "*",
           "include_global_state": true
         }'

echo "Restore request sent. Monitor Elasticsearch logs or use _cat/recovery to check restore progress."
