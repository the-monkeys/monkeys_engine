#!/usr/bin/env bash
set -euo pipefail

set -x

# Ensure jq is installed
if ! command -v jq &>/dev/null; then
    echo "jq is required but not installed. Please install jq before running this script."
    exit 1
fi

ES_USER="username"
ES_PASS="password"
REPO_NAME="my_backup"
ES_URL="http://localhost:9200"
SNAPSHOT_NAME="snapshot_$(date +%Y-%m-%d_%H-%M-%S)"

# Variables for cloning
LOCAL_REPO_PATH="/mnt/backups/es-backup"
REMOTE_USER="vboxuser"
REMOTE_HOST="192.168.1.14"
REMOTE_PATH="/home/vboxuser/"
REMOTE_PASS="changeme"
GOOGLE_DRIVE_DIR_ID="1H5aj73buIKLeIIAvrilBhFUTaGQZqVqI" # Adjust as needed

# Create the snapshot
curl -s -u "${ES_USER}:${ES_PASS}" -X PUT "${ES_URL}/_snapshot/${REPO_NAME}/${SNAPSHOT_NAME}?wait_for_completion=true" \
     -H 'Content-Type: application/json' \
     -d '{
           "indices": "*",
           "ignore_unavailable": true,
           "include_global_state": true
         }'

# Get a list of existing snapshots, sorted by start_time
SNAPSHOTS=$(curl -s -u "${ES_USER}:${ES_PASS}" -X GET "${ES_URL}/_snapshot/${REPO_NAME}/_all" \
            | jq -r '.snapshots | sort_by(.start_time) | .[].snapshot')

# Count how many snapshots we have
COUNT=$(echo "${SNAPSHOTS}" | wc -l | tr -d ' ')
echo "Found ${COUNT} snapshots"

# If we have more than 10 snapshots, delete the oldest ones
if [ "${COUNT}" -gt 10 ]; then
    # Calculate how many to delete
    TO_DELETE=$((COUNT - 10))

    # Get the oldest snapshots to delete
    OLD_SNAPSHOTS=$(echo "${SNAPSHOTS}" | head -n "${TO_DELETE}")

    # Delete each old snapshot
    for SNAP in ${OLD_SNAPSHOTS}; do
        curl -s -u "${ES_USER}:${ES_PASS}" -X DELETE "${ES_URL}/_snapshot/${REPO_NAME}/${SNAP}" \
             -H 'Content-Type: application/json'
    done
fi


echo "Starting scp..."
sshpass -p "${REMOTE_PASS}" scp -r "${LOCAL_REPO_PATH}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}"
echo "scp completed, now starting rclone..."

rclone copy "${LOCAL_REPO_PATH}" "gdrive:${GOOGLE_DRIVE_DIR_ID}" -v -r
echo "rclone completed."