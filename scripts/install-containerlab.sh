#!/bin/bash
# install-containerlab.sh - Install containerlab on Linux system

set -e

echo "Installing Containerlab"

# Check if running on Linux
if [[ "$OSTYPE" != "linux-gnu"* ]]; then
    echo "Error: Containerlab requires Linux"
    exit 1
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker first."
    echo "Visit: https://docs.docker.com/engine/install/"
    exit 1
fi

# Check if containerlab is already installed
if command -v containerlab &> /dev/null; then
    CURRENT_VERSION=$(containerlab version | grep "version:" | awk '{print $2}')
    echo "Containerlab $CURRENT_VERSION is already installed."
    read -p "Do you want to upgrade? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Skipping installation."
        exit 0
    fi
fi

echo "Installing containerlab using the official installation script..."
echo ""

# Use the official installation script
bash -c "$(curl -sL https://get.containerlab.dev)"

echo ""
echo "Containerlab Installation Complete"

# Check installation
if command -v containerlab &> /dev/null; then
    INSTALLED_VERSION=$(containerlab version | grep "version:" | awk '{print $2}')
    echo "✓ Containerlab version $INSTALLED_VERSION installed successfully"
    echo ""
    echo "Quick commands:"
    echo "  - Deploy lab:    sudo containerlab deploy -t topology.clab.yml"
    echo "  - Destroy lab:   sudo containerlab destroy -t topology.clab.yml"
    echo "  - Inspect lab:   sudo containerlab inspect"
    echo "  - Graph lab:     sudo containerlab graph -t topology.clab.yml"
else
    echo "✗ Installation failed"
    exit 1
fi

# Check if user needs to be added to docker group
if ! groups $USER | grep -q '\bdocker\b'; then
    echo ""
    echo "Note: Your user is not in the 'docker' group."
    echo "Run: sudo usermod -aG docker $USER"
    echo "Then logout and login again for sudo-less docker operation."
fi

echo ""
echo "Documentation: https://containerlab.dev/"
