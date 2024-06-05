#!/bin/bash

MIGRATE_VERSION="v4.14.1"
MIGRATE_URL="https://github.com/golang-migrate/migrate/releases/download/$MIGRATE_VERSION/migrate.linux-amd64.tar.gz"

# Create a temporary directory to work in
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Download the Golang Migrate tar.gz package
curl -L "$MIGRATE_URL" | tar xvz

# Check if the binary exists
if [ -f "migrate.linux-amd64" ]; then
    # Move the binary to a location on your system path (e.g., /usr/local/bin)
    sudo mv migrate.linux-amd64 /usr/local/bin/migrate
    echo "Golang Migrate installed successfully!"
else
    echo "Error: Golang Migrate binary not found."
fi

# Clean up temporary files
rm -rf "$TMP_DIR"
