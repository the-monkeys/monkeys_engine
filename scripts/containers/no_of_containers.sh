#!/bin/bash

set -x


# List all running containers and count the number of lines
num_containers=$(docker ps | wc -l)

# Subtract 1 from the count to exclude the header line
num_containers=$((num_containers - 1))

echo "There are currently $num_containers running containers."