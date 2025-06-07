#!/bin/bash
# scripts/test-connector.sh - Test connector scripts

set -euo pipefail

CONNECTOR_DIR="/etc/fail2ban/connectors"
TEST_IP="192.168.1.100"
TEST_JAIL="test"

echo "🧪 Testing fail2ban-notify connectors..."

# Test environment variables
export F2B_IP="$TEST_IP"
export F2B_JAIL="$TEST_JAIL"
export F2B_ACTION="ban"
export F2B_TIME=$(date -Iseconds)
export F2B_TIMESTAMP=$(date +%s)
export F2B_COUNTRY="Test Country"
export F2B_REGION="Test Region"
export F2B_CITY="Test City"
export F2B_ISP="Test ISP"
export F2B_FAILURES="5"

# Test URLs (using httpbin for testing)
export DISCORD_WEBHOOK_URL="https://httpbin.org/post"
export TEAMS_WEBHOOK_URL="https://httpbin.org/post"
export SLACK_WEBHOOK_URL="https://httpbin.org/post"
export TELEGRAM_BOT_TOKEN="123456789:test"
export TELEGRAM_CHAT_ID="12345"

# Find all connector scripts
if [ ! -d "$CONNECTOR_DIR" ]; then
    echo "❌ Connector directory not found: $CONNECTOR_DIR"
    exit 1
fi

connectors=$(find "$CONNECTOR_DIR" -type f -executable -name "*.sh" -o -name "*.py")

if [ -z "$connectors" ]; then
    echo "❌ No executable connectors found in $CONNECTOR_DIR"
    exit 1
fi

echo "Found connectors:"
echo "$connectors" | sed 's/^/  - /'
echo ""

# Test each connector
failed=0
for connector in $connectors; do
    name=$(basename "$connector")
    echo "Testing $name..."
    
    if timeout 30 "$connector"; then
        echo "✅ $name: PASSED"
    else
        echo "❌ $name: FAILED"
        ((failed++))
    fi
    echo ""
done

if [ $failed -eq 0 ]; then
    echo "🎉 All connectors passed!"
    exit 0
else
    echo "💥 $failed connector(s) failed!"
    exit 1
fi
