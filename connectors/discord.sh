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

# Check for jq (required for safe JSON construction)
if ! command -v jq &>/dev/null; then
    echo "Error: jq is required but not installed. Install it with: apt-get install jq" >&2
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
HOSTNAME_VAR="${F2B_HOSTNAME:-}"
FAILURES="${F2B_FAILURES:-0}"

# Determine color based on action
if [[ "$ACTION" == "unban" ]]; then
    COLOR=4505434   # Green
    EMOJI="✅"
    ACTION_DISPLAY="Unban"
else
    COLOR=16711684  # Red
    EMOJI="🚫"
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

# Build fields array safely using jq
FIELDS=$(jq -n \
    --arg ip "$IP" \
    --arg jail "$JAIL" \
    --arg action "$ACTION_DISPLAY" \
    '[
        {"name": "IP Address", "value": $ip, "inline": true},
        {"name": "Jail", "value": $jail, "inline": true},
        {"name": "Action", "value": $action, "inline": true}
    ]')

if [[ "$FAILURES" -gt 0 ]]; then
    FIELDS=$(echo "$FIELDS" | jq --arg val "$FAILURES" '. + [{"name": "Failures", "value": $val, "inline": true}]')
fi

if [[ -n "$ISP" ]]; then
    FIELDS=$(echo "$FIELDS" | jq --arg val "$ISP" '. + [{"name": "ISP", "value": $val, "inline": true}]')
fi

if [[ -n "$HOSTNAME_VAR" ]]; then
    FIELDS=$(echo "$FIELDS" | jq --arg val "$HOSTNAME_VAR" '. + [{"name": "Server", "value": $val, "inline": true}]')
fi

if [[ -n "$COUNTRY" ]]; then
    LOC_VAL="${CITY:+$CITY, }$COUNTRY"
    FIELDS=$(echo "$FIELDS" | jq --arg val "$LOC_VAL" '. + [{"name": "Location", "value": $val, "inline": true}]')
fi

# Create the payload safely using jq
DESCRIPTION="IP **${IP}**${LOCATION} has been ${ACTION}ned"
PAYLOAD=$(jq -n \
    --arg username "$USERNAME" \
    --arg avatar_url "$AVATAR_URL" \
    --arg title "$EMOJI Fail2Ban ${ACTION_DISPLAY}: $JAIL" \
    --arg description "$DESCRIPTION" \
    --argjson color "$COLOR" \
    --arg timestamp "$TIME" \
    --argjson fields "$FIELDS" \
    '{
        "username": $username,
        "avatar_url": $avatar_url,
        "embeds": [{
            "title": $title,
            "description": $description,
            "color": $color,
            "timestamp": $timestamp,
            "fields": $fields,
            "footer": {
                "text": "Fail2Ban Security Alert"
            }
        }]
    }')

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
