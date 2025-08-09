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

if curl -s -X PUT "http://localhost:9200/_snapshot/backup_repo" \
     -H "Content-Type: application/json" \
     -d "$REPO_BODY" > /dev/null; then
    echo -e "${GREEN}Repository registered successfully!${NC}"
else
    echo -e "${RED}Failed to register repository${NC}"
    exit 1
fi

# Check available snapshots
echo -e "${CYAN}Checking available snapshots...${NC}"
SNAPSHOTS_RESPONSE=$(curl -s "http://localhost:9200/_snapshot/backup_repo/_all?pretty")

if [ $? -eq 0 ]; then
    # Extract the latest snapshot name using jq if available, otherwise use basic parsing
    if command -v jq >/dev/null 2>&1; then
        LATEST_SNAPSHOT=$(echo "$SNAPSHOTS_RESPONSE" | jq -r '.snapshots[-1].snapshot // empty')
    else
        # Fallback parsing without jq
        LATEST_SNAPSHOT=$(echo "$SNAPSHOTS_RESPONSE" | grep '"snapshot"' | tail -1 | cut -d'"' -f4)
    fi
    
    if [ -n "$LATEST_SNAPSHOT" ] && [ "$LATEST_SNAPSHOT" != "null" ]; then
        echo -e "${GREEN}Latest snapshot found: $LATEST_SNAPSHOT${NC}"
        
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
        echo -e "${RED}No snapshots found in the repository!${NC}"
        exit 1
    fi
else
    echo -e "${RED}Failed to check snapshots${NC}"
    exit 1
fi

echo -e "${GREEN}All operations completed successfully!${NC}"
