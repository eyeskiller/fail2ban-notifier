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
HOSTNAME="${F2B_HOSTNAME:-}"
FAILURES="${F2B_FAILURES:-0}"

# Determine color and emoji based on action
if [[ "$ACTION" == "unban" ]]; then
    COLOR="good"  # Green
    EMOJI="✅"
else
    COLOR="danger"  # Red
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

# Create fields array
FIELDS='[
    {"title": "IP Address", "value": "'"$IP"'", "short": true},
    {"title": "Jail", "value": "'"$JAIL"'", "short": true},
    {"title": "Action", "value": "'"${ACTION^}"'", "short": true},
    {"title": "Time", "value": "'"$TIME"'", "short": true}'

if [[ "$FAILURES" -gt 0 ]]; then
    FIELDS+=',{"title": "Failures", "value": "'"$FAILURES"'", "short": true}'
fi

if [[ -n "$ISP" ]]; then
    FIELDS+=',{"title": "ISP", "value": "'"$ISP"'", "short": true}'
fi

if [[ -n "$HOSTNAME" ]]; then
    FIELDS+=',{"title": "Server", "value": "'"$HOSTNAME"'", "short": true}'
fi

if [[ -n "$COUNTRY" ]]; then
    FIELDS+=',{"title": "Location", "value": "'"${CITY:+$CITY, }$COUNTRY"'", "short": true}'
fi

FIELDS+=']'

# Create the payload
PAYLOAD=$(cat <<EOF
{
    "channel": "$CHANNEL",
    "username": "$USERNAME",
    "icon_emoji": "$ICON_EMOJI",
    "attachments": [{
        "color": "$COLOR",
        "title": "$EMOJI Fail2Ban ${ACTION^} Alert",
        "text": "IP *$IP*$LOCATION has been ${ACTION}ned in jail '$JAIL'",
        "fields": $FIELDS,
        "ts": $TIMESTAMP,
        "footer": "Fail2Ban Notifier",
        "footer_icon": "https://cdn-icons-png.flaticon.com/512/1828/1828506.png",
        "mrkdwn_in": ["text"],
        "actions": [{
            "type": "button",
            "text": "Check IP",
            "url": "https://whatismyipaddress.com/ip/$IP"
        }]
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
    echo "Slack notification sent successfully (HTTP $HTTP_CODE)"
    exit 0
else
    echo "Slack notification failed (HTTP $HTTP_CODE)" >&2
    exit 1
fi
