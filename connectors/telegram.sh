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

# Check for jq (required for safe JSON construction)
if ! command -v jq &>/dev/null; then
    echo "Error: jq is required but not installed. Install it with: apt-get install jq" >&2
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
HOSTNAME_VAR="${F2B_HOSTNAME:-}"
FAILURES="${F2B_FAILURES:-0}"

# Determine emoji based on action
if [[ "$ACTION" == "unban" ]]; then
    EMOJI="✅"
    ACTION_EMOJI="🔓"
    ACTION_DISPLAY="Unban"
else
    EMOJI="🚫"
    ACTION_EMOJI="🔒"
    ACTION_DISPLAY="Ban"
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

TIME_FORMATTED=$(date -d "$TIME" '+%Y-%m-%d %H:%M:%S %Z' 2>/dev/null || echo "$TIME")

# Create the message
MESSAGE="$EMOJI *Fail2Ban ${ACTION_DISPLAY} Alert*

🌐 *IP:* \`$IP_ESCAPED\`$LOCATION_ESCAPED
$ACTION_EMOJI *Jail:* $JAIL_ESCAPED
⚡ *Action:* ${ACTION_DISPLAY}
🕐 *Time:* $TIME_FORMATTED"

if [[ "$FAILURES" -gt 0 ]]; then
    MESSAGE="$MESSAGE
❌ *Failures:* $FAILURES"
fi

if [[ -n "$ISP" ]]; then
    ISP_ESCAPED=$(escape_markdown "$ISP")
    MESSAGE="$MESSAGE
🏢 *ISP:* $ISP_ESCAPED"
fi

if [[ -n "$HOSTNAME_VAR" ]]; then
    HOSTNAME_ESCAPED=$(escape_markdown "$HOSTNAME_VAR")
    MESSAGE="$MESSAGE
🖥️ *Server:* $HOSTNAME_ESCAPED"
fi

# Build reply_markup safely using jq if banning
REPLY_MARKUP="null"
if [[ "$ACTION" == "ban" ]]; then
    CHECK_URL="https://whatismyipaddress.com/ip/$IP"
    IPINFO_URL="https://ipinfo.io/$IP"
    REPLY_MARKUP=$(jq -n \
        --arg url1 "$CHECK_URL" \
        --arg url2 "$IPINFO_URL" \
        '{
            "inline_keyboard": [[
                {
                    "text": "🔍 Check IP",
                    "url": $url1
                },
                {
                    "text": "📊 IP Info",
                    "url": $url2
                }
            ]]
        }')
fi

# Create the payload safely using jq
PAYLOAD=$(jq -n \
    --arg chat_id "$CHAT_ID" \
    --arg text "$MESSAGE" \
    --argjson reply_markup "$REPLY_MARKUP" \
    '{
        "chat_id": $chat_id,
        "text": $text,
        "parse_mode": "Markdown",
        "disable_web_page_preview": true,
        "disable_notification": false
    }')

if [[ "$REPLY_MARKUP" != "null" ]]; then
    PAYLOAD=$(echo "$PAYLOAD" | jq --argjson rm "$REPLY_MARKUP" '. + {"reply_markup": $rm}')
fi

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
