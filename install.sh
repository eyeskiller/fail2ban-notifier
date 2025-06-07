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

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$SCRIPT_DIR"

# Check if binary exists
if [ ! -f "build/fail2ban-notify" ]; then
    echo "Error: Binary not found in build directory."
    echo "Please run 'make build' first to create the binary."
    exit 1
fi

# Install binary
echo "Installing fail2ban-notify binary..."
install -m 755 build/fail2ban-notify /usr/local/bin/

# Create necessary directories
echo "Creating configuration directories..."
mkdir -p /etc/fail2ban/action.d
mkdir -p /etc/fail2ban/connectors

# Install configuration files
echo "Installing configuration files..."
install -m 644 configs/notify.conf /etc/fail2ban/action.d/
install -m 644 configs/notify-enhanced.conf /etc/fail2ban/action.d/
install -m 644 configs/jail.local.example /etc/fail2ban/

# Install connector scripts
echo "Installing connector scripts..."
for connector in connectors/*; do
    install -m 755 "$connector" /etc/fail2ban/connectors/
done

# Initialize configuration
echo "Initializing configuration..."
/usr/local/bin/fail2ban-notify -init || echo "Could not initialize config (may need manual setup)"

# Prepare for cleanup
CLEANUP_SCRIPT=$(mktemp)
chmod +x "$CLEANUP_SCRIPT"

# Check if we're in a git repository
if [ -d ".git" ]; then
    echo "Detected installation from git repository."

    # Store the current directory path for cleanup
    REPO_DIR="$SCRIPT_DIR"

    # Create a cleanup script that will run after this script exits
    cat > "$CLEANUP_SCRIPT" << EOF
#!/bin/bash
echo "Performing cleanup..."
rm -rf "$REPO_DIR"
echo "Removed source directory from $REPO_DIR"
rm -f "\$0"  # Self-delete this cleanup script
EOF

    echo "Will remove source directory after installation."
    # Schedule the cleanup to run after this script exits
    trap "$CLEANUP_SCRIPT" EXIT
else
    # No cleanup needed, remove the cleanup script
    rm -f "$CLEANUP_SCRIPT"
fi

echo "=== Installation complete! ==="
echo "fail2ban-notifier has been installed to /usr/local/bin/fail2ban-notify"
echo "Configuration file has been initialized at /etc/fail2ban/fail2ban-notify.json"
echo "fail2ban actions have been installed at:"
echo "  - /etc/fail2ban/action.d/notify.conf (standard)"
echo "  - /etc/fail2ban/action.d/notify-enhanced.conf (enhanced)"
echo "Connector scripts have been installed to /etc/fail2ban/connectors/"
echo "Example jail configuration has been installed at /etc/fail2ban/jail.local.example"
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
