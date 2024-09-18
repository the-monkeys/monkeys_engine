#!/bin/bash

# Exit script on any error
set -e

# Function to install Go (latest version)
install_go() {
  echo "Installing Go (latest version)..."
  
  # Get the latest Go version download URL
  GO_VERSION=$(curl -s https://go.dev/VERSION?m=text)
  wget "https://dl.google.com/go/${GO_VERSION}.linux-amd64.tar.gz"

  # Remove any existing Go installation and extract the new one
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "${GO_VERSION}.linux-amd64.tar.gz"
  
  # Add Go to the system PATH (add this line to ~/.profile or ~/.bashrc for persistence)
  export PATH=$PATH:/usr/local/go/bin
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

  # Clean up
  rm "${GO_VERSION}.linux-amd64.tar.gz"
  
  # Verify installation
  go version
}

# Function to install Make
install_make() {
  echo "Installing Make..."
  sudo apt-get update
  sudo apt-get install -y make
}

# Function to install Protocol Buffers (protoc)
install_protoc() {
  echo "Installing Protocol Buffers (protoc)..."

  # Get the latest version of protoc (change version number if necessary)
  PROTOC_VERSION=$(curl -s https://api.github.com/repos/protocolbuffers/protobuf/releases/latest | grep -Po '"tag_name": "\K.*?(?=")')
  
  # Download protoc for Linux (replace 'linux-x86_64' with appropriate architecture if needed)
  wget "https://github.com/protocolbuffers/protobuf/releases/download/${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip"
  
  # Unzip and install
  unzip "protoc-${PROTOC_VERSION}-linux-x86_64.zip" -d protoc3
  sudo mv protoc3/bin/* /usr/local/bin/
  sudo mv protoc3/include/* /usr/local/include/
  
  # Clean up
  rm -rf protoc3 "protoc-${PROTOC_VERSION}-linux-x86_64.zip"
  
  # Verify installation
  protoc --version
}

# Function to install yq (YAML processor)
install_yq() {
  echo "Installing yq..."
  
  # Download the latest yq binary and make it executable
  sudo wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -O /usr/bin/yq
  sudo chmod +x /usr/bin/yq
  
  # Verify installation
  yq --version
}

# Function to install protoc-gen-go (Go plugin for Protocol Buffers)
install_protoc_gen_go() {
  echo "Installing protoc-gen-go (Go plugin for Protocol Buffers)..."
  
  # Use Go to install protoc-gen-go
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  
  # Add Go binaries to the system PATH (needed to access protoc-gen-go)
  export PATH=$PATH:$(go env GOPATH)/bin
  echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc

  # Verify installation
  protoc-gen-go --version
}

# Run installation functions
install_go
# install_make
# install_protoc
# install_yq
install_protoc_gen_go

echo "All tools installed successfully!"
