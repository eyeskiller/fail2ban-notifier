#!/bin/bash
# Slack Connector for fail2ban-notify
# Place this file in /etc/fail2ban/connectors/slack.sh

set -euo pipefail

# Configuration
WEBHOOK_URL="${SLACK_WEBHOOK_URL:-}"
CHANNEL="${SLACK_CHANNEL:-#security}"
USERNAME="${SLACK_USERNAME:-fail2ban}"
ICON_EMOJI="${SLACK_ICON_EMOJI:-:cop:}"

# Validation
if [[ -z "$WEBHOOK_URL" ]]; then
    echo "Error: SLACK_WEBHOOK_URL not set" >&2
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
TIMESTAMP="${F2B_TIMESTAMP:-$(date +%s)}"
COUNTRY="${F2B_COUNTRY:-}"
REGION="${F2B_REGION:-}"
CITY="${F2B_CITY:-}"
ISP="${F2B_ISP:-}"
HOSTNAME_VAR="${F2B_HOSTNAME:-}"
FAILURES="${F2B_FAILURES:-0}"

# Determine color and emoji based on action
if [[ "$ACTION" == "unban" ]]; then
    COLOR="good"  # Green
    EMOJI="✅"
    ACTION_DISPLAY="Unban"
else
    COLOR="danger"  # Red
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
    --arg time "$TIME" \
    '[
        {"title": "IP Address", "value": $ip, "short": true},
        {"title": "Jail", "value": $jail, "short": true},
        {"title": "Action", "value": $action, "short": true},
        {"title": "Time", "value": $time, "short": true}
    ]')

if [[ "$FAILURES" -gt 0 ]]; then
    FIELDS=$(echo "$FIELDS" | jq --arg val "$FAILURES" '. + [{"title": "Failures", "value": $val, "short": true}]')
fi

if [[ -n "$ISP" ]]; then
    FIELDS=$(echo "$FIELDS" | jq --arg val "$ISP" '. + [{"title": "ISP", "value": $val, "short": true}]')
fi

if [[ -n "$HOSTNAME_VAR" ]]; then
    FIELDS=$(echo "$FIELDS" | jq --arg val "$HOSTNAME_VAR" '. + [{"title": "Server", "value": $val, "short": true}]')
fi

if [[ -n "$COUNTRY" ]]; then
    LOC_VAL="${CITY:+$CITY, }$COUNTRY"
    FIELDS=$(echo "$FIELDS" | jq --arg val "$LOC_VAL" '. + [{"title": "Location", "value": $val, "short": true}]')
fi

# Create the payload safely using jq
TEXT="IP *${IP}*${LOCATION} has been ${ACTION}ned in jail '${JAIL}'"
CHECK_URL="https://whatismyipaddress.com/ip/${IP}"
PAYLOAD=$(jq -n \
    --arg channel "$CHANNEL" \
    --arg username "$USERNAME" \
    --arg icon_emoji "$ICON_EMOJI" \
    --arg color "$COLOR" \
    --arg title "$EMOJI Fail2Ban ${ACTION_DISPLAY} Alert" \
    --arg text "$TEXT" \
    --argjson fields "$FIELDS" \
    --argjson ts "$TIMESTAMP" \
    --arg check_url "$CHECK_URL" \
    '{
        "channel": $channel,
        "username": $username,
        "icon_emoji": $icon_emoji,
        "attachments": [{
            "color": $color,
            "title": $title,
            "text": $text,
            "fields": $fields,
            "ts": $ts,
            "footer": "Fail2Ban Notifier",
            "footer_icon": "https://cdn-icons-png.flaticon.com/512/1828/1828506.png",
            "mrkdwn_in": ["text"],
            "actions": [{
                "type": "button",
                "text": "Check IP",
                "url": $check_url
            }]
        }]
    }')

# Send the notification
HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$WEBHOOK_URL")

if [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]; then
    echo "Slack notification sent successfully (HTTP $HTTP_CODE)"
    exit 0
else
    echo "Slack notification failed (HTTP $HTTP_CODE)" >&2
    exit 1
fi
