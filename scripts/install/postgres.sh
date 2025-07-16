#!/bin/bash

# Install PostgreSQL
sudo apt update
sudo apt install postgresql postgresql-contrib

# Prompt for database name, username, and password
read -p "Enter the name of the database: " dbname
read -p "Enter the username: " username
read -s -p "Enter the password: " password
echo

# Create a new database
sudo -u postgres createdb $dbname

# Create a new user
sudo -u postgres createuser --createdb --login --superuser $username

# Set a password for the user
sudo -u postgres psql -c "ALTER USER $username WITH PASSWORD '$password';"

# Grant permission on the database to the user
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE $dbname TO $username;"

echo "PostgreSQL installation and configuration complete!"
