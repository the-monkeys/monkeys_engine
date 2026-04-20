#!/usr/bin/env bash
set -euo pipefail

PROTOC_VER="34.1"
GO_VER="1.24.2"

echo "==> Upgrading protoc to v${PROTOC_VER} and Go protoc plugins to latest..."

# 1. Install latest protoc
echo "--- Installing protoc v${PROTOC_VER} ---"
curl -fsSL -o /tmp/protoc.zip "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VER}/protoc-${PROTOC_VER}-linux-x86_64.zip"
sudo rm -rf /usr/local/protoc
sudo mkdir -p /usr/local/protoc
sudo unzip -q -o /tmp/protoc.zip -d /usr/local/protoc
sudo chmod +x /usr/local/protoc/bin/protoc
sudo ln -sf /usr/local/protoc/bin/protoc /usr/local/bin/protoc
rm -f /tmp/protoc.zip

# Remove old apt version if present
sudo apt-get remove -y protobuf-compiler 2>/dev/null || true

echo "protoc version: $(/usr/local/bin/protoc --version)"

# 2. Ensure Go is installed (needed for go install)
if ! command -v go &>/dev/null; then
    echo "--- Installing Go ${GO_VER} ---"
    curl -fsSL -o /tmp/go.tar.gz "https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz
fi

# Ensure Go is on PATH for this script
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
echo "Go version: $(go version)"

# 3. Install latest protoc-gen-go and protoc-gen-go-grpc
echo "--- Installing latest protoc-gen-go ---"
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

echo "--- Installing latest protoc-gen-go-grpc ---"
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Ensure ~/go/bin is on PATH
export PATH="$HOME/go/bin:$PATH"

# 4. Verify
echo ""
echo "=== Installation complete ==="
echo "protoc:              $(/usr/local/bin/protoc --version)"
echo "protoc-gen-go:       $(protoc-gen-go --version 2>&1 || echo 'installed')"
echo "protoc-gen-go-grpc:  $(protoc-gen-go-grpc --version 2>&1 || echo 'installed')"
echo "go:                  $(go version)"
echo ""
echo "Make sure these are in your PATH in ~/.bashrc:"
echo '  export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"'
