#!/bin/bash
# PagerDuty Connector for fail2ban-notify
# Place this file in /etc/fail2ban/connectors/pagerduty.sh

set -euo pipefail

# Configuration
ROUTING_KEY="${PAGERDUTY_ROUTING_KEY:-}"
API_URL="https://events.pagerduty.com/v2/enqueue"

# Validation
if [[ -z "$ROUTING_KEY" ]]; then
    echo "Error: PAGERDUTY_ROUTING_KEY not set" >&2
    exit 1
fi

# Check for jq
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

# Determine event action and severity
if [[ "$ACTION" == "unban" ]]; then
    EVENT_ACTION="resolve"
    SEVERITY="info"
    ACTION_DISPLAY="Unban"
else
    EVENT_ACTION="trigger"
    SEVERITY="critical"
    ACTION_DISPLAY="Ban"
fi

# Build summary and dedup key
SUMMARY="Fail2Ban ${ACTION_DISPLAY}: ${IP} in ${JAIL}${HOSTNAME_VAR:+ on $HOSTNAME_VAR}"
DEDUP_KEY="fail2ban-${JAIL}-${IP}"

# Build location details
LOCATION=""
if [[ -n "$COUNTRY" ]]; then
    LOCATION="${CITY:+$CITY, }$COUNTRY"
fi

# Build the PagerDuty Events API v2 payload
PAYLOAD=$(jq -n \
    --arg routing_key "$ROUTING_KEY" \
    --arg event_action "$EVENT_ACTION" \
    --arg dedup_key "$DEDUP_KEY" \
    --arg summary "$SUMMARY" \
    --arg source "$IP" \
    --arg severity "$SEVERITY" \
    --arg component "$JAIL" \
    --arg time "$TIME" \
    --arg location "$LOCATION" \
    --arg isp "$ISP" \
    --arg hostname "$HOSTNAME_VAR" \
    --arg failures "$FAILURES" \
    '{
        "routing_key": $routing_key,
        "event_action": $event_action,
        "dedup_key": $dedup_key,
        "client": "Fail2Ban Notifier",
        "client_url": "https://github.com/eyeskiller/fail2ban-notifier",
        "payload": {
            "summary": $summary,
            "source": $source,
            "severity": $severity,
            "timestamp": $time,
            "component": $component,
            "group": "fail2ban",
            "class": "security",
            "custom_details": {}
        }
    }')

# Add custom details if available
if [[ -n "$LOCATION" ]]; then
    PAYLOAD=$(echo "$PAYLOAD" | jq --arg val "$LOCATION" '.payload.custom_details.location = $val')
fi

if [[ -n "$ISP" ]]; then
    PAYLOAD=$(echo "$PAYLOAD" | jq --arg val "$ISP" '.payload.custom_details.isp = $val')
fi

if [[ -n "$HOSTNAME_VAR" ]]; then
    PAYLOAD=$(echo "$PAYLOAD" | jq --arg val "$HOSTNAME_VAR" '.payload.custom_details.server = $val')
fi

if [[ "$ACTION" != "unban" && "$FAILURES" -gt 0 ]]; then
    PAYLOAD=$(echo "$PAYLOAD" | jq --arg val "$FAILURES" '.payload.custom_details.failures = $val')
fi

# Add links
PAYLOAD=$(echo "$PAYLOAD" | jq \
    --arg check_url "https://whatismyipaddress.com/ip/${IP}" \
    '.links = [{"href": $check_url, "text": "Check IP"}]')

# Send the notification
HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$API_URL")

if [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]; then
    echo "PagerDuty notification sent successfully (HTTP $HTTP_CODE)"
    exit 0
else
    echo "PagerDuty notification failed (HTTP $HTTP_CODE)" >&2
    exit 1
fi
