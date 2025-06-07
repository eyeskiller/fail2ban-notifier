#!/bin/bash
# scripts/install-dev.sh - Development installation script

set -euo pipefail

echo "🔧 Installing fail2ban-notify for development..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.19 or later."
    exit 1
fi

# Build the project
echo "🏗️ Building project..."
make build

# Install binary
echo "📦 Installing binary..."
sudo install -m 755 dist/fail2ban-notify /usr/local/bin/

# Create connector directory
echo "📁 Creating connector directory..."
sudo mkdir -p /etc/fail2ban/connectors

# Install connectors
echo "🔌 Installing connectors..."
sudo cp connectors/*.sh /etc/fail2ban/connectors/ 2>/dev/null || true
sudo cp connectors/*.py /etc/fail2ban/connectors/ 2>/dev/null || true
sudo chmod +x /etc/fail2ban/connectors/*

# Install fail2ban action config
if [ -d /etc/fail2ban/action.d ]; then
    echo "⚙️ Installing fail2ban action..."
    sudo cp configs/notify.conf /etc/fail2ban/action.d/
else
    echo "⚠️ Fail2ban not found, skipping action installation"
fi

# Install helper scripts
echo "🛠️ Installing helper scripts..."
sudo cp scripts/create-connector.sh /usr/local/bin/
sudo chmod +x /usr/local/bin/create-connector.sh

# Initialize configuration
echo "📝 Initializing configuration..."
sudo /usr/local/bin/fail2ban-notify -init

echo "✅ Development installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit configuration: sudo nano /etc/fail2ban/fail2ban-notify.json"
echo "2. Test installation: sudo fail2ban-notify -ip=1.2.3.4 -jail=test -debug"
echo "3. Test connectors: sudo scripts/test-connector.sh"