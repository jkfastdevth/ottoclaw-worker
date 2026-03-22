#!/usr/bin/env bash
# Siam-Synapse Web Installer Launcher
# Usage: sudo bash install-web.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Root check
if [[ "$EUID" -ne 0 ]]; then
   echo "❌ Please run with sudo: sudo bash install-web.sh"
   exit 1
fi

echo "🚀 Starting Siam-Synapse Web Installer..."
echo "📂 Path: ${SCRIPT_DIR}/cmd/setup-web"

if ! command -v go &>/dev/null; then
    echo "❌ Go is required to run the web installer."
    echo ""
    echo "  Install Go on Linux/Debian:"
    echo "    sudo apt-get install -y golang-go"
    echo ""
    echo "  Or download from: https://go.dev/dl/"
    echo "    curl -OL https://go.dev/dl/go1.22.linux-amd64.tar.gz"
    echo "    sudo tar -C /usr/local -xzf go1.22.linux-amd64.tar.gz"
    echo "    export PATH=\$PATH:/usr/local/go/bin"
    exit 1
fi

if ! command -v git &>/dev/null; then
    echo "❌ Git is required for the Bootstrap installation process."
    echo "Please run: sudo apt-get update && sudo apt-get install -y git"
    exit 1
fi

cd "${SCRIPT_DIR}/cmd/setup-web"
echo "🌐 Web server is launching at: http://localhost:3333"
echo "📢 Please open this URL in your browser to continue the installation."
echo ""
echo "Press Ctrl+C to stop the installer after deployment is complete."
echo ""

go run main.go
