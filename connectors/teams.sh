#!/bin/bash
# Microsoft Teams Connector for fail2ban-notify
# Place this file in /etc/fail2ban/connectors/teams.sh

set -euo pipefail

# Configuration
WEBHOOK_URL="${TEAMS_WEBHOOK_URL:-}"

# Validation
if [[ -z "$WEBHOOK_URL" ]]; then
    echo "Error: TEAMS_WEBHOOK_URL not set" >&2
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
HOSTNAME="${F2B_HOSTNAME:-}"
FAILURES="${F2B_FAILURES:-0}"

# Determine color and emoji based on action
if [[ "$ACTION" == "unban" ]]; then
    THEME_COLOR="44FF44"  # Green
    EMOJI="✅"
else
    THEME_COLOR="FF4444"  # Red
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

# Create facts array
FACTS='[
    {"name": "IP Address", "value": "'"$IP"'"},
    {"name": "Jail", "value": "'"$JAIL"'"},
    {"name": "Action", "value": "'"${ACTION^}"'"},
    {"name": "Time", "value": "'"$TIME"'"}'

if [[ "$FAILURES" -gt 0 ]]; then
    FACTS+=',{"name": "Failures", "value": "'"$FAILURES"'"}'
fi

if [[ -n "$ISP" ]]; then
    FACTS+=',{"name": "ISP", "value": "'"$ISP"'"}'
fi

if [[ -n "$HOSTNAME" ]]; then
    FACTS+=',{"name": "Server", "value": "'"$HOSTNAME"'"}'
fi

if [[ -n "$COUNTRY" ]]; then
    FACTS+=',{"name": "Location", "value": "'"${CITY:+$CITY, }$COUNTRY"'"}'
fi

FACTS+=']'

# Create the payload
PAYLOAD=$(cat <<EOF
{
    "@type": "MessageCard",
    "@context": "http://schema.org/extensions",
    "themeColor": "$THEME_COLOR",
    "summary": "Fail2Ban ${ACTION^}: $IP",
    "sections": [{
        "activityTitle": "$EMOJI Fail2Ban ${ACTION^} Alert",
        "activitySubtitle": "IP $IP$LOCATION has been ${ACTION}ned in jail '$JAIL'",
        "activityImage": "https://cdn-icons-png.flaticon.com/512/1828/1828506.png",
        "facts": $FACTS,
        "markdown": true
    }],
    "potentialAction": [{
        "@type": "OpenUri",
        "name": "Check IP Details",
        "targets": [{
            "os": "default",
            "uri": "https://whatismyipaddress.com/ip/$IP"
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
    echo "Teams notification sent successfully (HTTP $HTTP_CODE)"
    exit 0
else
    echo "Teams notification failed (HTTP $HTTP_CODE)" >&2
    exit 1
fi
