#!/bin/bash
# Discord Connector for fail2ban-notify
# Place this file in /etc/fail2ban/connectors/discord.sh

set -euo pipefail

# Configuration - set via environment variables from main config
WEBHOOK_URL="${DISCORD_WEBHOOK_URL:-}"
USERNAME="${DISCORD_USERNAME:-Fail2Ban}"
AVATAR_URL="${DISCORD_AVATAR_URL:-}"

# Validation
if [[ -z "$WEBHOOK_URL" ]]; then
    echo "Error: DISCORD_WEBHOOK_URL not set" >&2
    exit 1
fi

# Read JSON data from stdin (optional - we also have env vars)
JSON_DATA=""
if [[ -p /dev/stdin ]]; then
    JSON_DATA=$(cat)
fi

# Get data from environment variables (set by main program)
IP="${F2B_IP:-unknown}"
JAIL="${F2B_JAIL:-unknown}"
ACTION="${F2B_ACTION:-ban}"
TIME="${F2B_TIME:-$(date -Iseconds)}"
COUNTRY="${F2B_COUNTRY:-}"
REGION="${F2B_REGION:-}"
CITY="${F2B_CITY:-}"
ISP="${F2B_ISP:-}"
FAILURES="${F2B_FAILURES:-0}"

# Determine color based on action
if [[ "$ACTION" == "unban" ]]; then
    COLOR="4505434"  # Green
    EMOJI="✅"
else
    COLOR="16711684"  # Red
    EMOJI="🚫"
fi

# Build location string
LOCATION=""
if [[ -n "$COUNTRY" ]]; then
    LOCATION=" from $COUNTRY"
    if [[ -n "$CITY" ]]; then
        LOCATION=" from $CITY, $COUNTRY"
    fi
fi

# Create the embed fields
FIELDS='[
    {"name": "IP Address", "value": "'"$IP"'", "inline": true},
    {"name": "Jail", "value": "'"$JAIL"'", "inline": true},
    {"name": "Action", "value": "'"${ACTION^}"'", "inline": true}'

if [[ "$FAILURES" -gt 0 ]]; then
    FIELDS+=',{"name": "Failures", "value": "'"$FAILURES"'", "inline": true}'
fi

if [[ -n "$ISP" ]]; then
    FIELDS+=',{"name": "ISP", "value": "'"$ISP"'", "inline": true}'
fi

if [[ -n "$COUNTRY" ]]; then
    FIELDS+=',{"name": "Location", "value": "'"${CITY:+$CITY, }$COUNTRY"'", "inline": true}'
fi

FIELDS+=']'

# Create the payload
PAYLOAD=$(cat <<EOF
{
    "username": "$USERNAME",
    "avatar_url": "$AVATAR_URL",
    "embeds": [{
        "title": "$EMOJI Fail2Ban ${ACTION^}: $JAIL",
        "description": "IP **$IP**$LOCATION has been ${ACTION}ned",
        "color": $COLOR,
        "timestamp": "$TIME",
        "fields": $FIELDS,
        "footer": {
            "text": "Fail2Ban Security Alert"
        }
    }]
}
EOF
)

# Send the notification
HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$WEBHOOK_URL")

if [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]; then
    echo "Discord notification sent successfully (HTTP $HTTP_CODE)"
    exit 0
else
    echo "Discord notification failed (HTTP $HTTP_CODE)" >&2
    exit 1
fi