#!/bin/bash

# git pull origin main
git pull origin main

# remove docker-compose.yaml file and config\config.yaml
rm docker-compose.yaml config/config.yaml

# copy docker-compose.yaml and config/config.yaml from user's home directory
cp ~/monkeys_engine/docker-compose.yaml .
cp ~/monkeys_engine/config/config.yaml config/

# restart docker containers
docker-compose down
docker-compose up -d    

# wait for the containers to be up
echo "Waiting for containers to start..."
sleep 10