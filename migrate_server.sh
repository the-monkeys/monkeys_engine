#!/bin/bash
#
# This script migrates the entire Docker environment (containers, volumes, images)
# for this project to a new server.
#
# INSTRUCTIONS:
# 1. Fill in the NEW_SERVER_USER and NEW_SERVER_IP variables below.
# 2. Make sure you have passwordless SSH access to the new server (e.g., using SSH keys).
# 3. Run this script from the root of your project directory on the OLD server.
#    chmod +x migrate_server.sh
#    ./migrate_server.sh
#

# --- CONFIGURATION ---
# ==> SET THESE VALUES
NEW_SERVER_USER="root"
NEW_SERVER_IP="192.168.1.100"
# <== END OF VALUES TO SET

# --- SCRIPT ---
set -e # Exit immediately if a command exits with a non-zero status.

# Variables
SSH_TARGET="${NEW_SERVER_USER}@${NEW_SERVER_IP}"
BACKUP_DIR="/tmp/docker_migration_backup_$$"
PROJECT_ARCHIVE="project_files.tar.gz"
IMAGES_ARCHIVE="docker_images.tar"
PROJECT_NAME=$(basename "$PWD")

echo "--- Starting Docker Migration to ${SSH_TARGET} ---"

# --- 1. Backup on the OLD server ---
echo "[OLD SERVER] Creating temporary backup directory: ${BACKUP_DIR}"
mkdir -p "${BACKUP_DIR}"

echo "[OLD SERVER] Stopping services..."
docker-compose down

echo "[OLD SERVER] Saving Docker images used by docker-compose..."
docker-compose images -q | uniq | xargs docker save -o "${BACKUP_DIR}/${IMAGES_ARCHIVE}"

echo "[OLD SERVER] Backing up Docker volumes..."
for volume in $(docker-compose config --volumes); do
    echo "  -> Backing up volume: ${volume}"
    docker run --rm -v "${volume}:/data" -v "${BACKUP_DIR}:/backup" alpine \
        tar czf "/backup/${volume}.tar.gz" -C /data .
done

echo "[OLD SERVER] Archiving project directory..."
tar czf "${BACKUP_DIR}/${PROJECT_ARCHIVE}" --exclude-from=.dockerignore --exclude=".git" --exclude="${BACKUP_DIR}" .

# --- 2. Transfer to the NEW server ---
echo "[TRANSFER] Copying backups to new server..."
scp -r "${BACKUP_DIR}" "${SSH_TARGET}:/tmp/"

# --- 3. Restore on the NEW server ---
echo "[NEW SERVER] Running remote restoration script..."
ssh "${SSH_TARGET}" << EOF
    set -e
    REMOTE_BACKUP_DIR="${BACKUP_DIR}"
    REMOTE_PROJECT_DIR="~/${PROJECT_NAME}"

    echo "[NEW SERVER] Creating project directory: \${REMOTE_PROJECT_DIR}"
    mkdir -p "\${REMOTE_PROJECT_DIR}"

    echo "[NEW SERVER] Loading Docker images..."
    docker load -i "\${REMOTE_BACKUP_DIR}/${IMAGES_ARCHIVE}"

    echo "[NEW SERVER] Restoring project files..."
    tar xzf "\${REMOTE_BACKUP_DIR}/${PROJECT_ARCHIVE}" -C "\${REMOTE_PROJECT_DIR}"

    echo "[NEW SERVER] Restoring Docker volumes..."
    for volume_backup in \${REMOTE_BACKUP_DIR}/*.tar.gz; do
        if [[ "\$volume_backup" != *"${PROJECT_ARCHIVE}"* ]]; then
            volume_name=\$(basename "\$volume_backup" .tar.gz)
            echo "  -> Restoring volume: \${volume_name}"
            docker volume create "\${volume_name}" > /dev/null
            docker run --rm -v "\${volume_name}:/data" -v "\${REMOTE_BACKUP_DIR}:/backup" alpine \\
                tar xzf "/backup/\${volume_name}.tar.gz" -C /data
        fi
    done

    echo "[NEW SERVER] Starting services with docker-compose..."
    cd "\${REMOTE_PROJECT_DIR}"
    docker-compose up -d

    echo "[NEW SERVER] Cleaning up temporary backup files..."
    rm -rf "\${REMOTE_BACKUP_DIR}"

    echo "[NEW SERVER] --- Restoration Complete ---"
EOF

# --- 4. Cleanup on the OLD server ---
echo "[OLD SERVER] Cleaning up local backup files..."
rm -rf "${BACKUP_DIR}"

echo "--- Migration Script Finished Successfully! ---"
