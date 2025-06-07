#!/bin/bash
# Telegram Connector for fail2ban-notify
# Place this file in /etc/fail2ban/connectors/telegram.sh

set -euo pipefail

# Configuration
BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
CHAT_ID="${TELEGRAM_CHAT_ID:-}"

# Validation
if [[ -z "$BOT_TOKEN" ]]; then
    echo "Error: TELEGRAM_BOT_TOKEN not set" >&2
    exit 1
fi

if [[ -z "$CHAT_ID" ]]; then
    echo "Error: TELEGRAM_CHAT_ID not set" >&2
    exit 1
fi

# Get data from environment variables
IP="${F2B_IP:-unknown}"
JAIL="${F2B_JAIL:-unknown}"
ACTION="${F2B_ACTION:-ban}"
TIME="${F2B_TIME:-$(date -Iseconds)}"
COUNTRY="${F2B_COUNTRY:-}"
REGION="${F2B_REGION:-}"
CITY="${F2B_CITY:-}"
ISP="${F2B_ISP:-}"
FAILURES="${F2B_FAILURES:-0}"

# Determine emoji based on action
if [[ "$ACTION" == "unban" ]]; then
    EMOJI="✅"
    ACTION_EMOJI="🔓"
else
    EMOJI="🚫"
    ACTION_EMOJI="🔒"
fi

# Build location string
LOCATION=""
if [[ -n "$COUNTRY" ]]; then
    LOCATION=" from $COUNTRY"
    if [[ -n "$CITY" ]]; then
        LOCATION=" from $CITY, $COUNTRY"
    fi
fi

# Escape special characters for Markdown
escape_markdown() {
    echo "$1" | sed 's/[[\*_`]/\\&/g'
}

IP_ESCAPED=$(escape_markdown "$IP")
JAIL_ESCAPED=$(escape_markdown "$JAIL")
LOCATION_ESCAPED=$(escape_markdown "$LOCATION")

# Create the message
MESSAGE="$EMOJI *Fail2Ban ${ACTION^} Alert*

🌐 *IP:* \`$IP_ESCAPED\`$LOCATION_ESCAPED
$ACTION_EMOJI *Jail:* $JAIL_ESCAPED
⚡ *Action:* ${ACTION^}
🕐 *Time:* $(date -d "$TIME" '+%Y-%m-%d %H:%M:%S %Z' 2>/dev/null || echo "$TIME")"

if [[ "$FAILURES" -gt 0 ]]; then
    MESSAGE="$MESSAGE
❌ *Failures:* $FAILURES"
fi

if [[ -n "$ISP" ]]; then
    ISP_ESCAPED=$(escape_markdown "$ISP")
    MESSAGE="$MESSAGE
🏢 *ISP:* $ISP_ESCAPED"
fi

# Add action buttons
INLINE_KEYBOARD=""
if [[ "$ACTION" == "ban" ]]; then
    INLINE_KEYBOARD='"reply_markup": {
        "inline_keyboard": [[
            {
                "text": "🔍 Check IP",
                "url": "https://whatismyipaddress.com/ip/'$IP'"
            },
            {
                "text": "📊 IP Info",
                "url": "https://ipinfo.io/'$IP'"
            }
        ]]
    },'
fi

# Create the payload
PAYLOAD=$(cat <<EOF
{
    "chat_id": "$CHAT_ID",
    "text": "$MESSAGE",
    "parse_mode": "Markdown",
    "disable_web_page_preview": true,
    $INLINE_KEYBOARD
    "disable_notification": false
}
EOF
)

# API URL
API_URL="https://api.telegram.org/bot$BOT_TOKEN/sendMessage"

# Send the notification
RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$API_URL")

# Parse response
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$RESPONSE" | head -n -1)

if [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]; then
    echo "Telegram notification sent successfully (HTTP $HTTP_CODE)"
    exit 0
else
    echo "Telegram notification failed (HTTP $HTTP_CODE)" >&2
    echo "Response: $RESPONSE_BODY" >&2
    exit 1
fi