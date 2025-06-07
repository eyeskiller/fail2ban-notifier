#!/bin/bash
# scripts/build-deb.sh - Build Debian package

set -euo pipefail

VERSION=${1:-"1.0.0"}
ARCH=${2:-"amd64"}
PACKAGE_NAME="fail2ban-notify"
BUILD_DIR="build/package/deb"
PACKAGE_DIR="$BUILD_DIR/$PACKAGE_NAME-$VERSION"

echo "🏗️ Building Debian package v$VERSION for $ARCH..."

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$PACKAGE_DIR"/{DEBIAN,usr/{local/bin,share/doc/$PACKAGE_NAME},etc/fail2ban/{action.d,connectors}}

# Copy binary
cp "dist/$PACKAGE_NAME-linux-$ARCH" "$PACKAGE_DIR/usr/local/bin/$PACKAGE_NAME"
chmod 755 "$PACKAGE_DIR/usr/local/bin/$PACKAGE_NAME"

# Copy configurations
cp configs/notify.conf "$PACKAGE_DIR/etc/fail2ban/action.d/"
cp connectors/*.sh "$PACKAGE_DIR/etc/fail2ban/connectors/" 2>/dev/null || true
cp connectors/*.py "$PACKAGE_DIR/etc/fail2ban/connectors/" 2>/dev/null || true
chmod +x "$PACKAGE_DIR/etc/fail2ban/connectors/"*

# Copy scripts
cp scripts/create-connector.sh "$PACKAGE_DIR/usr/local/bin/"
chmod +x "$PACKAGE_DIR/usr/local/bin/create-connector.sh"

# Copy documentation
cp README.md "$PACKAGE_DIR/usr/share/doc/$PACKAGE_NAME/"
cp LICENSE "$PACKAGE_DIR/usr/share/doc/$PACKAGE_NAME/" 2>/dev/null || echo "LICENSE file not found"

# Create control file
cat > "$PACKAGE_DIR/DEBIAN/control" << EOF
Package: $PACKAGE_NAME
Version: $VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Depends: fail2ban, curl
Recommends: python3
Maintainer: Your Name <your-email@example.com>
Description: Modular notification system for Fail2Ban
 A modern, modular notification system for Fail2Ban written in Go.
 Send real-time security alerts to Discord, Microsoft Teams, Slack,
 Telegram, email, or any custom service through pluggable connectors.
 .
 Features:
  - Modular architecture with external connector scripts
  - Multi-platform support (Discord, Teams, Slack, Telegram, Email)
  - Geographic information with automatic IP geolocation
  - Easy configuration with JSON-based config
  - Custom connector creation tools
Homepage: https://github.com/eyeskiller/fail2ban-notifier
EOF

# Create postinst script
cat > "$PACKAGE_DIR/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e

# Initialize configuration if it doesn't exist
if [ ! -f /etc/fail2ban/fail2ban-notify.json ]; then
    echo "Initializing fail2ban-notify configuration..."
    /usr/local/bin/fail2ban-notify -init
fi

# Restart fail2ban if it's running
if systemctl is-active --quiet fail2ban; then
    echo "Restarting fail2ban service..."
    systemctl restart fail2ban
fi

echo "fail2ban-notify installed successfully!"
echo "Edit /etc/fail2ban/fail2ban-notify.json to configure your notification services."
EOF

# Create prerm script
cat > "$PACKAGE_DIR/DEBIAN/prerm" << 'EOF'
#!/bin/bash
set -e

# Stop fail2ban before removal to prevent errors
if systemctl is-active --quiet fail2ban; then
    echo "Stopping fail2ban service..."
    systemctl stop fail2ban
fi
EOF

# Create postrm script
cat > "$PACKAGE_DIR/DEBIAN/postrm" << 'EOF'
#!/bin/bash
set -e

case "$1" in
    purge)
        # Remove configuration files on purge
        rm -f /etc/fail2ban/fail2ban-notify.json
        rm -rf /etc/fail2ban/connectors
        ;;
    remove|upgrade|failed-upgrade|abort-install|abort-upgrade|disappear)
        # Start fail2ban if it was stopped
        if command -v systemctl >/dev/null 2>&1; then
            systemctl start fail2ban 2>/dev/null || true
        fi
        ;;
esac
EOF

# Make scripts executable
chmod 755 "$PACKAGE_DIR/DEBIAN"/{postinst,prerm,postrm}

# Build package
dpkg-deb --build "$PACKAGE_DIR" "$BUILD_DIR/${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"

echo "✅ Debian package created: $BUILD_DIR/${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
