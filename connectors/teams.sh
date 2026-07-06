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

# Determine color and emoji based on action
if [[ "$ACTION" == "unban" ]]; then
    THEME_COLOR="44FF44"  # Green
    EMOJI="✅"
    ACTION_DISPLAY="Unban"
else
    THEME_COLOR="FF4444"  # Red
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

# Build facts array safely using jq
FACTS=$(jq -n \
    --arg ip "$IP" \
    --arg jail "$JAIL" \
    --arg action "$ACTION_DISPLAY" \
    --arg time "$TIME" \
    '[
        {"name": "IP Address", "value": $ip},
        {"name": "Jail", "value": $jail},
        {"name": "Action", "value": $action},
        {"name": "Time", "value": $time}
    ]')

if [[ "$FAILURES" -gt 0 ]]; then
    FACTS=$(echo "$FACTS" | jq --arg val "$FAILURES" '. + [{"name": "Failures", "value": $val}]')
fi

if [[ -n "$ISP" ]]; then
    FACTS=$(echo "$FACTS" | jq --arg val "$ISP" '. + [{"name": "ISP", "value": $val}]')
fi

if [[ -n "$HOSTNAME_VAR" ]]; then
    FACTS=$(echo "$FACTS" | jq --arg val "$HOSTNAME_VAR" '. + [{"name": "Server", "value": $val}]')
fi

if [[ -n "$COUNTRY" ]]; then
    LOC_VAL="${CITY:+$CITY, }$COUNTRY"
    FACTS=$(echo "$FACTS" | jq --arg val "$LOC_VAL" '. + [{"name": "Location", "value": $val}]')
fi

# Create the payload safely using jq
SUMMARY="Fail2Ban ${ACTION_DISPLAY}: ${IP}"
ACTIVITY_TITLE="${EMOJI} Fail2Ban ${ACTION_DISPLAY} Alert"
ACTIVITY_SUBTITLE="IP ${IP}${LOCATION} has been ${ACTION}ned in jail '${JAIL}'"
CHECK_URL="https://whatismyipaddress.com/ip/${IP}"

PAYLOAD=$(jq -n \
    --arg theme_color "$THEME_COLOR" \
    --arg summary "$SUMMARY" \
    --arg act_title "$ACTIVITY_TITLE" \
    --arg act_subtitle "$ACTIVITY_SUBTITLE" \
    --argjson facts "$FACTS" \
    --arg check_url "$CHECK_URL" \
    '{
        "@type": "MessageCard",
        "@context": "http://schema.org/extensions",
        "themeColor": $theme_color,
        "summary": $summary,
        "sections": [{
            "activityTitle": $act_title,
            "activitySubtitle": $act_subtitle,
            "activityImage": "https://cdn-icons-png.flaticon.com/512/1828/1828506.png",
            "facts": $facts,
            "markdown": true
        }],
        "potentialAction": [{
            "@type": "OpenUri",
            "name": "Check IP Details",
            "targets": [{
                "os": "default",
                "uri": $check_url
            }]
        }]
    }')

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
