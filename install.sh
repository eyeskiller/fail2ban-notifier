#!/bin/bash
set -e

# fail2ban-notifier installer script
echo "=== fail2ban-notifier installer ==="
echo "This script will install fail2ban-notifier on your system."

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo."
    exit 1
fi

# Check for required dependencies
echo "Checking dependencies..."
command -v go >/dev/null 2>&1 || { echo "Error: Go is required but not installed. Please install Go first."; exit 1; }
command -v git >/dev/null 2>&1 || { echo "Error: Git is required but not installed. Please install Git first."; exit 1; }

# Create temporary directory
TEMP_DIR=$(mktemp -d)
echo "Created temporary directory: $TEMP_DIR"
cd "$TEMP_DIR"

# Clone repository
echo "Cloning repository..."
git clone https://github.com/eyeskiller/fail2ban-notifier.git
cd fail2ban-notifier

# Build
echo "Building fail2ban-notifier..."
make build

# Install
echo "Installing fail2ban-notifier..."
make install

# Clean up
echo "Cleaning up..."
cd /
rm -rf "$TEMP_DIR"

echo "=== Installation complete! ==="
echo "fail2ban-notifier has been installed to /usr/local/bin/fail2ban-notify"
echo "Configuration file has been initialized at /etc/fail2ban/fail2ban-notify.json"
echo "fail2ban action has been installed at /etc/fail2ban/action.d/notify.conf"
echo ""
echo "Next steps:"
echo "1. Configure your notification services by editing /etc/fail2ban/fail2ban-notify.json"
echo "2. Test your configuration: fail2ban-notify -status"
echo "3. Add the 'notify' action to your fail2ban jail.local file"
echo "   Example: action = iptables-multiport[name=ssh, port=\"ssh\", protocol=tcp]"
echo "            notify[name=ssh]"
echo ""
echo "For more information, see the documentation at:"
echo "https://github.com/eyeskiller/fail2ban-notifier"