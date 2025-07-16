#!/bin/bash

set -x

# Get a list of all container IDs
container_ids=$(docker ps -aq)

# Remove all containers
if [ ! -z "$container_ids" ]
then
  docker rm -f $container_ids
  echo "Removed all containers."
else
  echo "No containers to remove."
fi

# Define the container name and image name
CONTAINER_NAME=hungry_galileo
IMAGE_NAME=postgres:12-alpine

# OpenSearch Params
OPENSEARCH_CONTAINER=condescending_monkey
OPENSEARCH_IMAGE=opensearchproject/opensearch:latest


# Set the Postgres password
POSTGRES_USER=root
POSTGRES_PASSWORD=Secret
POSTGRES_DB=the_monkeys

# Create and run the container
docker run --restart always -d --name $CONTAINER_NAME \
    -e POSTGRES_USER=$POSTGRES_USER \
    -e POSTGRES_PASSWORD=$POSTGRES_PASSWORD \
    -e POSTGRES_DB=$POSTGRES_DB \
    -p 5432:5432 \
    $IMAGE_NAME


sudo docker run --restart always -d --name $OPENSEARCH_CONTAINER -p 9200:9200 -p 9600:9600 -e "discovery.type=single-node" $OPENSEARCH_IMAGE


echo "Docker containers have been created and running!"


# Install Golang Migrate
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.15.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/migrate
sudo chmod +x /usr/local/bin/migrate

migrate -path psql/migration -database "postgresql://$POSTGRES_USER:$POSTGRES_PASSWORD@127.0.0.1:5432/$POSTGRES_DB?sslmode=disable" -verbose up


echo "All migrations completed successfully."



