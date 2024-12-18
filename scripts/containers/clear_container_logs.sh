#!/bin/bash

# Base directory where container data is stored
base_dir="/var/snap/docker/common/var-lib-docker/containers"

# Check if the base directory exists
if [ ! -d "$base_dir" ]; then
    echo "Directory $base_dir not found. Ensure Docker is installed and logs are in the default location."
    exit 1
fi

# Loop through each subdirectory
for dir in "$base_dir"/*; do
    # Check if it is a directory
    if [ -d "$dir" ]; then
        # Get the directory name
        dir_name=$(basename "$dir")

        # Path to the JSON log file named after the directory
        hash_file="$dir/$dir_name-json.log"

        # Check if the file exists
        if [ -f "$hash_file" ]; then
            echo "Processing file: $hash_file"

            # Create a backup of the log file
            backup_file="$hash_file.backup"
            cp "$hash_file" "$backup_file"
            echo "Backup created: $backup_file"

            # Truncate the original log file
            sudo truncate -s 0 "$hash_file"
            echo "File truncated: $hash_file"
        else
            echo "File $dir_name-json.log not found in directory: $dir"
        fi
    fi
done

echo "Script execution completed."