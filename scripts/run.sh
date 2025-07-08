#!/bin/bash

echo "Stashing any local changes..."
git stash

echo "Stopping Docker Compose services..."
docker-compose stop

read -p "Enter the branch you want to deploy: " branch_name

current_branch=$(git rev-parse --abbrev-ref HEAD)

if [ "$current_branch" == "$branch_name" ]; then
    echo "Already on branch $branch_name. Pulling the latest code..."
    git pull origin "$branch_name"
else
    echo "Switching to branch $branch_name..."
    git fetch origin
    git checkout "$branch_name"
    git pull origin "$branch_name"
fi

echo "Applying stashed changes..."
git stash pop || echo "No stash found to apply."

echo "Starting Docker Compose services..."
docker-compose up -d

echo "Deployment complete."
