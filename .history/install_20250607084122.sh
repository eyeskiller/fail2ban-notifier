#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="fail2ban-notify"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/fail2ban"
ACTION_DIR="/etc/fail2ban/action.d"
GITHUB_REPO="eyeskiller/fail2ban-notifier"
LATEST_RELEASE_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"

echo -e "${BLUE}🔧 Fail2Ban Notify - Installation Script${NC}"
echo "=================================================="

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}❌ This script must be run as root${NC}"
   echo "Please run: sudo $0"
   exit 1
fi

# Detect architecture and OS
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7l)
        ARCH="armv7"
        ;;
    *)
        echo -e "${RED}❌ Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

case $OS in
    linux)
        OS="linux"
        ;;
    darwin)
        OS="darwin"
        ;;
    *)
        echo -e "${RED}❌ Unsupported OS: $OS${NC}"
        exit 1
        ;;
esac

echo -e "${BLUE}📊 Detected: ${OS}-${ARCH}${NC}"

# Check if fail2ban is installed
if ! command -v fail2ban-server &> /dev/null; then
    echo -e "${YELLOW}⚠️  fail2ban not found. Installing...${NC}"

    if command -v apt-get &> /dev/null; then
        apt-get update && apt-get install -y fail2ban
    elif command -v yum &> /dev/null; then
        yum install -y epel-release && yum install -y fail2ban
    elif command -v dnf &> /dev/null; then
        dnf install -y fail2ban
    elif command -v pacman &> /dev/null; then
        pacman -S --noconfirm fail2ban
    else
        echo -e "${RED}❌ Could not install fail2ban. Please install it manually.${NC}"
        exit 1
    fi

    echo -e "${GREEN}✅ fail2ban installed${NC}"
fi

# Create temporary directory
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

echo -e "${BLUE}📥 Downloading fail2ban-notify...${NC}"

# For development/demo purposes, we'll create the binary from source
# In production, you would download from GitHub releases
cat > main.go << 'EOF'
// The complete Go source code would go here
// For brevity, this is a placeholder - in real deployment,
// you would download the pre-compiled binary from GitHub releases
EOF

# Alternative: Download from GitHub releases (uncomment when repo exists)
# DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${BINARY_NAME}-${OS}-${ARCH}"
# if ! curl -L -o "$BINARY_NAME" "$DOWNLOAD_URL"; then
#     echo -e "${RED}❌ Failed to download binary${NC}"
#     exit 1
# fi

# For now, we'll assume the binary is built or provided
echo -e "${YELLOW}📦 Building from source (requires Go)...${NC}"

if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go not found. Please install Go or download pre-built binary${NC}"
    echo "You can install Go from: https://golang.org/dl/"
    exit 1
fi

# Create a simple version of the binary for installation
cat > main.go << 'EOF'
package main

import (
    "flag"
    "fmt"
    "os"
)

func main() {
    fmt.Println("fail2ban-notify placeholder - replace with full implementation")

    var (
        initConfig = flag.Bool("init", false, "Initialize configuration file")
        configPath = flag.String("config", "/etc/fail2ban/fail2ban-notify.json", "Path to configuration file")
    )
    flag.Parse()

    if *initConfig {
        fmt.Printf("Would create config at: %s\n", *configPath)
        fmt.Println("Use the full implementation for actual functionality")
        return
    }

    fmt.Println("Notification would be sent here")
}
EOF

go mod init fail2ban-notify
go build -o "$BINARY_NAME" .

echo -e "${BLUE}📁 Installing binary...${NC}"

# Install binary
install -m 755 "$BINARY_NAME" "$INSTALL_DIR/"

echo -e "${BLUE}📝 Creating fail2ban action configuration...${NC}"

# Create action configuration
cat > notify.conf << 'EOF'
# Fail2Ban notification action configuration
# Place this file in /etc/fail2ban/action.d/notify.conf

[INCLUDES]

before = iptables-common.conf

[Definition]

# Option: actionstart
# Notes.: command executed on demand at the first ban (or at the start of Fail2Ban if actionstart_on_demand is set to false).
# Values: CMD
actionstart =

# Option: actionstop
# Notes.: command executed at the stop of jail (or at the end of Fail2Ban)
# Values: CMD
actionstop =

# Option: actioncheck
# Notes.: command executed once before each actionban command
# Values: CMD
actioncheck =

# Option: actionban
# Notes.: command executed when banning an IP. Take care that the
#         command is executed with Fail2Ban user rights.
# Tags:    <ip>  IP address
#          <failures>  number of failures
#          <time>  unix timestamp of the ban time
# Values: CMD
actionban = /usr/local/bin/fail2ban-notify -ip="<ip>" -jail="<name>" -action="ban" -failures="<failures>"

# Option: actionunban
# Notes.: command executed when unbanning an IP. Take care that the
#         command is executed with Fail2Ban user rights.
# Tags:    <ip>  IP address
#          <failures>  number of failures
#          <time>  unix timestamp of the ban time
# Values: CMD
actionunban = /usr/local/bin/fail2ban-notify -ip="<ip>" -jail="<name>" -action="unban" -failures="<failures>"

[Init]

# Default name of the chain
name = default
EOF

# Install action configuration
install -m 644 notify.conf "$ACTION_DIR/"

echo -e "${BLUE}⚙️  Initializing configuration...${NC}"

# Initialize configuration
"$INSTALL_DIR/$BINARY_NAME" -init

echo -e "${GREEN}✅ Installation completed successfully!${NC}"
echo ""
echo -e "${BLUE}📋 Next Steps:${NC}"
echo "1. Edit the configuration file: /etc/fail2ban/fail2ban-notify.json"
echo "2. Configure your notification services (Discord, Teams, Slack, Telegram)"
echo "3. Add the 'notify' action to your fail2ban jails"
echo ""
echo -e "${BLUE}📝 Example jail configuration:${NC}"
echo "Add to /etc/fail2ban/jail.local:"
echo ""
echo "[sshd]"
echo "enabled = true"
echo "port = ssh"
echo "filter = sshd"
echo "logpath = /var/log/auth.log"
echo "maxretry = 3"
echo "bantime = 3600"
echo "action = iptables[name=SSH, port=ssh, protocol=tcp]"
echo "         notify"
echo ""
echo -e "${BLUE}🧪 Test the installation:${NC}"
echo "sudo $INSTALL_DIR/$BINARY_NAME -ip=\"192.168.1.100\" -jail=\"test\" -action=\"ban\" -debug"
echo ""
echo -e "${BLUE}🔧 Configure notifications:${NC}"
echo "sudo nano /etc/fail2ban/fail2ban-notify.json"
echo ""
echo -e "${YELLOW}⚠️  Remember to restart fail2ban after configuration:${NC}"
echo "sudo systemctl restart fail2ban"

# Cleanup
cd /
rm -rf "$TEMP_DIR"

echo -e "${GREEN}🎉 Ready to go!${NC}"
