#!/bin/bash
set -e

# fail2ban-notifier installer script
# Downloads the latest pre-built binary from GitHub releases
echo "===== fail2ban-notifier installer ====="
echo ""

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo."
    exit 1
fi

# Check if fail2ban-notify is already installed
if [ -f "/usr/local/bin/fail2ban-notify" ]; then
    echo "fail2ban-notify is already installed."
    read -p "Do you want to reinstall it? (y/n): " choice
    case "$choice" in
        y|Y ) echo "Proceeding with reinstallation...";;
        * ) echo "Installation cancelled."; exit 0;;
    esac
fi

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64)  GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l|armv8l)  GOARCH="armv7" ;;
    i386|i686)     GOARCH="386" ;;
    *)
        echo "Unsupported architecture: $ARCH. Building from source instead."
        echo "Install Go and run: make build && sudo make install"
        exit 1
        ;;
esac

# Only Linux binaries are published
if [ "$OS" != "linux" ]; then
    echo "No pre-built binary for $OS. Building from source instead."
    echo "Install Go and run: make build && sudo make install"
    exit 1
fi

# Get the latest release tag
echo "Fetching latest release info..."
LATEST=$(curl -sL https://api.github.com/repos/eyeskiller/fail2ban-notifier/releases/latest 2>/dev/null || echo "")
TAG=$(echo "$LATEST" | grep '"tag_name"' | head -1 | cut -d'"' -f4)

if [ -z "$TAG" ]; then
    echo "Could not determine latest release. Using version from VERSION file."
    TAG=$(curl -sL https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/main/VERSION 2>/dev/null || echo "1.1.0")
fi

echo "Latest version: $TAG"

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
echo "Created temporary directory: $TEMP_DIR"

# Download the archive
ARCHIVE_NAME="fail2ban-notify_${TAG}_linux_${GOARCH}.tar.gz"
ARCHIVE_URL="https://github.com/eyeskiller/fail2ban-notifier/releases/download/${TAG}/${ARCHIVE_NAME}"

echo "Downloading $ARCHIVE_NAME..."
curl -sL -o "$TEMP_DIR/$ARCHIVE_NAME" "$ARCHIVE_URL" || {
    echo "Failed to download archive for linux/$GOARCH."
    echo "Available releases: https://github.com/eyeskiller/fail2ban-notifier/releases"
    rm -rf "$TEMP_DIR"
    exit 1
}

# Extract the archive
echo "Extracting archive..."
tar -xzf "$TEMP_DIR/$ARCHIVE_NAME" -C "$TEMP_DIR"
EXTRACT_DIR="$TEMP_DIR/fail2ban-notify_${TAG}_linux_${GOARCH}"

# Install binary
echo "Installing fail2ban-notify binary..."
install -m 755 "$EXTRACT_DIR/fail2ban-notify" /usr/local/bin/

# Create necessary directories
echo "Creating configuration directories..."
mkdir -p /etc/fail2ban/action.d
mkdir -p /etc/fail2ban/connectors

# Install configuration files
echo "Installing configuration files..."
for f in "$EXTRACT_DIR"/configs/*; do
    [ -f "$f" ] && install -m 644 "$f" /etc/fail2ban/action.d/
done
[ -f "$EXTRACT_DIR/configs/jail.local.example" ] && install -m 644 "$EXTRACT_DIR/configs/jail.local.example" /etc/fail2ban/

# Install connector scripts
echo "Installing connector scripts..."
for connector in "$EXTRACT_DIR"/connectors/*; do
    [ -f "$connector" ] && install -m 755 "$connector" /etc/fail2ban/connectors/
done

# Initialize configuration
echo "Initializing configuration..."
/usr/local/bin/fail2ban-notify -init || echo "Could not initialize config (may need manual setup)"

# Cleanup
echo "Cleaning up..."
rm -rf "$TEMP_DIR"

echo ""
echo "====== Installation complete! ======"
echo "fail2ban-notifier has been installed to /usr/local/bin/fail2ban-notify"
echo "Configuration file has been initialized at /etc/fail2ban/fail2ban-notify.json"
echo ""
echo "Next steps:"
echo "  1. Configure your notification services by editing /etc/fail2ban/fail2ban-notify.json"
echo "  2. Test your configuration: fail2ban-notify -status"
echo "  3. Add the 'notify' action to your fail2ban jail.local file"
echo ""
echo "For more information: https://github.com/eyeskiller/fail2ban-notifier"
