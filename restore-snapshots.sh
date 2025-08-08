#!/bin/bash

# Bash script to restore Elasticsearch snapshots from backup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
GRAY='\033[0;37m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting Elasticsearch snapshot restoration process...${NC}"

# Wait for Elasticsearch to be ready
echo -e "${YELLOW}Waiting for Elasticsearch to be ready...${NC}"
while true; do
    if curl -s "http://localhost:9200/_cluster/health" > /dev/null 2>&1; then
        break
    else
        echo -e "${GRAY}   Elasticsearch not ready yet, waiting 5 seconds...${NC}"
        sleep 5
    fi
done

echo -e "${GREEN}Elasticsearch is ready!${NC}"

# Register the snapshot repository
echo -e "${CYAN}Registering snapshot repository...${NC}"
REPO_BODY='{
    "type": "fs",
    "settings": {
        "location": "/usr/share/elasticsearch/snapshots"
    }
}'

# Register the snapshot repository
echo -e "${CYAN}Registering snapshot repository...${NC}"
REPO_BODY='{
    "type": "fs",
    "settings": {
        "location": "/usr/share/elasticsearch/snapshots"
    }
}'

REPO_RESPONSE=$(curl -s -X PUT "http://localhost:9200/_snapshot/backup_repo" \
     -H "Content-Type: application/json" \
     -d "$REPO_BODY")

if echo "$REPO_RESPONSE" | grep -q '"acknowledged".*true'; then
    echo -e "${GREEN}Repository registered successfully!${NC}"
else
    echo -e "${YELLOW}Repository registration response: $REPO_RESPONSE${NC}"
    # Check if repository already exists
    EXISTING_REPO=$(curl -s "http://localhost:9200/_snapshot/backup_repo")
    if echo "$EXISTING_REPO" | grep -q '"type".*"fs"'; then
        echo -e "${GREEN}Repository already exists and is configured correctly!${NC}"
    else
        echo -e "${RED}Failed to register repository${NC}"
        exit 1
    fi
fi

# Wait a moment for repository to be fully ready
sleep 2

# Check available snapshots
echo -e "${CYAN}Checking available snapshots...${NC}"

# First verify the repository is accessible
REPO_STATUS=$(curl -s "http://localhost:9200/_snapshot/backup_repo")
if echo "$REPO_STATUS" | grep -q "repository_missing_exception"; then
    echo -e "${RED}Repository is missing. Attempting to re-register...${NC}"
    curl -s -X PUT "http://localhost:9200/_snapshot/backup_repo" \
         -H "Content-Type: application/json" \
         -d "$REPO_BODY" > /dev/null
    sleep 2
fi

SNAPSHOTS_RESPONSE=$(curl -s "http://localhost:9200/_snapshot/backup_repo/_all?pretty")

# Debug: Show the raw response
echo -e "${GRAY}Snapshots API response:${NC}"
echo "$SNAPSHOTS_RESPONSE"

if echo "$SNAPSHOTS_RESPONSE" | grep -q "repository_missing_exception"; then
    echo -e "${RED}Repository is still missing after re-registration${NC}"
    exit 1
elif echo "$SNAPSHOTS_RESPONSE" | grep -q "snapshots.*\[\]"; then
    echo -e "${YELLOW}Repository exists but no snapshots found${NC}"
    echo -e "${YELLOW}Checking if snapshot files exist in filesystem...${NC}"
    # Check if there are snapshot files in the mounted directory
    docker exec elasticsearch-node1 ls -la /usr/share/elasticsearch/snapshots/ 2>/dev/null || echo -e "${RED}Cannot access snapshots directory in container${NC}"
    exit 1
elif echo "$SNAPSHOTS_RESPONSE" | grep -q '"snapshots"'; then
    # Extract the latest snapshot name using jq if available, otherwise use basic parsing
    if command -v jq >/dev/null 2>&1; then
        LATEST_SNAPSHOT=$(echo "$SNAPSHOTS_RESPONSE" | jq -r '.snapshots[-1].snapshot // empty')
    else
        # Fallback parsing without jq - improved to handle multiple snapshots
        LATEST_SNAPSHOT=$(echo "$SNAPSHOTS_RESPONSE" | grep '"snapshot"' | tail -1 | sed 's/.*"snapshot" : "\([^"]*\)".*/\1/')
    fi

    if [ -n "$LATEST_SNAPSHOT" ] && [ "$LATEST_SNAPSHOT" != "null" ] && [ "$LATEST_SNAPSHOT" != "empty" ]; then
        echo -e "${GREEN}Latest snapshot found: $LATEST_SNAPSHOT${NC}"

        # Show all available snapshots for reference
        echo -e "${CYAN}All available snapshots:${NC}"
        if command -v jq >/dev/null 2>&1; then
            echo "$SNAPSHOTS_RESPONSE" | jq -r '.snapshots[].snapshot'
        else
            echo "$SNAPSHOTS_RESPONSE" | grep '"snapshot"' | sed 's/.*"snapshot" : "\([^"]*\)".*/\1/'
        fi

        # Restore the latest snapshot
        echo -e "${CYAN}Restoring snapshot: $LATEST_SNAPSHOT${NC}"
        RESTORE_BODY='{
            "ignore_unavailable": true,
            "include_global_state": false
        }'
        if curl -s -X POST "http://localhost:9200/_snapshot/backup_repo/$LATEST_SNAPSHOT/_restore?pretty" \
             -H "Content-Type: application/json" \
             -d "$RESTORE_BODY" > /dev/null; then
            echo -e "${GREEN}Restoration initiated successfully!${NC}"

            echo -e "${YELLOW}Waiting for restoration to complete...${NC}"
            sleep 15

            # Check available indices
            echo -e "${CYAN}Checking available indices...${NC}"
            INDICES_OUTPUT=$(curl -s "http://localhost:9200/_cat/indices?v")
            echo -e "${WHITE}$INDICES_OUTPUT${NC}"
            echo -e "${GREEN}Snapshot restoration process completed!${NC}"
            echo -e "${GREEN}Your Elasticsearch data should now be available!${NC}"
        else
            echo -e "${RED}Failed to restore snapshot${NC}"
            exit 1
        fi
        else
            echo -e "${RED}No valid snapshots found in the repository!${NC}"
            exit 1
        fi
    else
        echo -e "${RED}Failed to parse snapshots response${NC}"
        echo -e "${GRAY}Raw response: $SNAPSHOTS_RESPONSE${NC}"
        exit 1
    fi

echo -e "${GREEN}All operations completed successfully!${NC}"
