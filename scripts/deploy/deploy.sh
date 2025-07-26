#!/bin/bash

set -e

# git pull origin main
git pull origin main

# remove docker-compose.yaml file and config\config.yaml
rm ~/monkeys-engine/docker-compose.yml ~/monkeys-engineconfig/config.yaml

# copy docker-compose.yml and config/config.yml from user's home directory
cp ~/docker-compose.yml ~/monkeys-engine/docker-compose.yml
cp ~/config.yaml ~/monkeys-engine/config/

# restart docker containers
sudo docker-compose down
sudo docker-compose up -d

# wait for the containers to be up
echo "Waiting for containers to start..."
sleep 10