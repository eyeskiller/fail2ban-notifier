# fail2ban-notifier

<div align="center">
  <img src="https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/master/assets/logo.png" alt="fail2ban-notifier logo" width="200" height="200" onerror="this.style.display='none'">

  <h3>A powerful notification system for Fail2Ban</h3>
  <p>Send alerts to various services when IPs are banned or unbanned</p>

  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
  [![GitHub stars](https://img.shields.io/github/stars/eyeskiller/fail2ban-notifier?style=social)](https://github.com/eyeskiller/fail2ban-notifier/stargazers)
</div>

## ✨ Features

- **🔔 Multiple Notification Services**: Support for Discord, Slack, Microsoft Teams, Telegram, Email, and custom webhooks
- **🌎 GeoIP Integration**: Automatically lookup and include geographic information about banned IPs
- **⚙️ Flexible Configuration**: Easy to configure and extend with new notification services
- **🔒 Fail2Ban Integration**: Seamlessly integrates with Fail2Ban's action system
- **📝 Customizable Templates**: Notification messages can be customized for each service
- **🔄 Retry Mechanism**: Built-in retry for failed notifications
- **🔍 Connector Discovery**: Automatically discovers available notification connectors
- **🧩 Extensible**: Create your own custom connectors to integrate with any service

## 📥 Installation

### One-liner Installation

```bash
curl -sSL https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/master/install.sh | sudo bash
```

### Manual Installation (Recommended)

1. Clone the repository:
   ```bash
   git clone https://github.com/eyeskiller/fail2ban-notifier.git
   cd fail2ban-notifier
   ```

2. Build and install:
   ```bash
   make build
   sudo ./install.sh
   ```

The installation script will:
- Install the binary to `/usr/local/bin/fail2ban-notify`
- Copy configuration files to `/etc/fail2ban/action.d/`
- Copy connector scripts to `/etc/fail2ban/connectors/`
- Initialize the configuration at `/etc/fail2ban/fail2ban-notify.json`

### Using Pre-built Binaries

If you have downloaded a release with pre-built binaries:

```bash
sudo ./install.sh
```

This will use the existing binary in the `build` directory and install all necessary files.

## ⚙️ Configuration

After installation, the configuration file is created at `/etc/fail2ban/fail2ban-notify.json`. You'll need to edit this file to enable and configure your notification services.

### Basic Configuration

```json
{
  "connectors": [
    {
      "name": "discord",
      "type": "script",
      "enabled": true,
      "path": "/etc/fail2ban/connectors/discord.sh",
      "settings": {
        "DISCORD_WEBHOOK_URL": "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN",
        "DISCORD_USERNAME": "Fail2Ban",
        "DISCORD_AVATAR_URL": ""
      },
      "timeout": 30,
      "retry_count": 2,
      "retry_delay": 5,
      "description": "Send notifications to Discord via webhook"
    }
  ],
  "connector_path": "/etc/fail2ban/connectors",
  "geoip": {
    "enabled": true,
    "service": "ipapi",
    "cache": true,
    "ttl": 3600
  },
  "debug": false,
  "log_level": "info",
  "timeout": 30
}
```

### 🔌 Enabling Connectors

To enable a connector:

1. Check available connectors:
   ```bash
   sudo fail2ban-notify -discover
   ```

2. Test a connector:
   ```bash
   sudo fail2ban-notify -test discord
   ```

3. Edit the configuration file to enable the connector:
   ```bash
   sudo nano /etc/fail2ban/fail2ban-notify.json
   ```
   Set `"enabled": true` for the connector you want to use.

### 🔒 Fail2Ban Integration

To integrate with Fail2Ban, add the `notify` action to your jail configuration:

1. Edit your jail.local file:
   ```bash
   sudo nano /etc/fail2ban/jail.local
   ```

2. Add the notify action to your jail:
   ```
   [ssh]
   enabled = true
   port = ssh
   filter = sshd
   logpath = /var/log/auth.log
   maxretry = 5
   bantime = 3600
   action = iptables-multiport[name=ssh, port="ssh", protocol=tcp]
            notify[name=ssh]
   ```

## 🛠️ Usage

### Command Line Reference

| Command | Description | Example |
|---------|-------------|---------|
| `-action string` | Action performed (ban/unban) | `-action="unban"` |
| `-config string` | Path to configuration file | `-config="/path/to/config.json"` |
| `-debug` | Enable debug logging | `-debug` |
| `-discover` | Discover available connectors | `-discover` |
| `-failures int` | Number of failures | `-failures=5` |
| `-init` | Initialize configuration file | `-init` |
| `-ip string` | IP address that was banned/unbanned | `-ip="192.168.1.100"` |
| `-jail string` | Fail2ban jail name | `-jail="ssh"` |
| `-status` | Show connector status | `-status` |
| `-test string` | Test specific connector | `-test="discord"` |
| `-version` | Show version information | `-version` |

### Common Examples

#### Discover Available Connectors
```bash
sudo fail2ban-notify -discover
```
This command scans the connector directory and displays all available connectors.

#### Test a Connector
```bash
sudo fail2ban-notify -test discord
```
This sends a test notification using the specified connector.

#### Check Connector Status
```bash
sudo fail2ban-notify -status
```
Shows the status of all configured connectors (enabled/disabled/invalid).

#### Manually Trigger a Notification
```bash
sudo fail2ban-notify -ip="192.168.1.100" -jail="ssh" -action="ban" -failures=5
```
Manually sends a notification to all enabled connectors.

#### Initialize Configuration
```bash
sudo fail2ban-notify -init
```
Creates a default configuration file with sample connectors.

## 🔔 Supported Notification Services

- **Discord**: Send notifications to Discord channels via webhooks
- **Slack**: Send notifications to Slack channels via webhooks
- **Microsoft Teams**: Send notifications to Teams channels via webhooks
- **Telegram**: Send notifications to Telegram chats via bot API
- **Email**: Send email notifications via SMTP
- **Custom Webhook**: Send notifications to any HTTP endpoint

## 🧩 Creating Custom Connectors

You can extend fail2ban-notifier by creating your own custom connectors to integrate with additional services. Connectors can be implemented as scripts (Bash, Python, etc.) or HTTP webhooks.

### Connector Types

1. **Script Connectors**: Executable scripts that receive notification data via environment variables and stdin
2. **HTTP Connectors**: Webhook endpoints that receive notification data as JSON payloads

### Creating a Script Connector

1. Create a new script file in the `/etc/fail2ban/connectors/` directory (e.g., `myservice.sh`)
2. Make the script executable: `chmod +x /etc/fail2ban/connectors/myservice.sh`
3. Implement your connector logic using the environment variables and stdin data

#### Script Connector Template (Bash)

```bash
#!/bin/bash
# MyService Connector for fail2ban-notify
# Place this file in /etc/fail2ban/connectors/myservice.sh

set -euo pipefail

# Configuration - set via environment variables from main config
API_KEY="${MYSERVICE_API_KEY:-}"
API_URL="${MYSERVICE_API_URL:-https://api.example.com/notify}"

# Validation
if [[ -z "$API_KEY" ]]; then
    echo "Error: MYSERVICE_API_KEY not set" >&2
    exit 1
fi

# Read JSON data from stdin
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

# Create your service-specific payload
PAYLOAD=$(cat <<EOF
{
    "event": "fail2ban_${ACTION}",
    "ip": "$IP",
    "jail": "$JAIL",
    "time": "$TIME",
    "location": "${CITY:+$CITY, }${COUNTRY}",
    "failures": $FAILURES
}
EOF
)

# Send the notification to your service
HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d "$PAYLOAD" \
    "$API_URL")

if [[ "$HTTP_CODE" -ge 200 && "$HTTP_CODE" -lt 300 ]]; then
    echo "MyService notification sent successfully (HTTP $HTTP_CODE)"
    exit 0
else
    echo "MyService notification failed (HTTP $HTTP_CODE)" >&2
    exit 1
fi
```

#### Script Connector Template (Python)

```python
#!/usr/bin/env python3
# MyService Connector for fail2ban-notify
# Place this file in /etc/fail2ban/connectors/myservice.py

import os
import sys
import json
import requests
from datetime import datetime

# Configuration from environment variables
api_key = os.environ.get('MYSERVICE_API_KEY', '')
api_url = os.environ.get('MYSERVICE_API_URL', 'https://api.example.com/notify')

# Validation
if not api_key:
    print("Error: MYSERVICE_API_KEY not set", file=sys.stderr)
    sys.exit(1)

# Get data from environment variables
ip = os.environ.get('F2B_IP', 'unknown')
jail = os.environ.get('F2B_JAIL', 'unknown')
action = os.environ.get('F2B_ACTION', 'ban')
time_str = os.environ.get('F2B_TIME', datetime.now().isoformat())
country = os.environ.get('F2B_COUNTRY', '')
region = os.environ.get('F2B_REGION', '')
city = os.environ.get('F2B_CITY', '')
isp = os.environ.get('F2B_ISP', '')
failures = int(os.environ.get('F2B_FAILURES', '0'))

# Read JSON data from stdin (optional)
try:
    if not sys.stdin.isatty():
        json_data = json.load(sys.stdin)
except:
    json_data = {}

# Create payload for your service
location = f"{city}, {country}" if city and country else country
payload = {
    "event": f"fail2ban_{action}",
    "ip": ip,
    "jail": jail,
    "time": time_str,
    "location": location,
    "failures": failures
}

# Send notification
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {api_key}"
}

try:
    response = requests.post(api_url, json=payload, headers=headers)
    response.raise_for_status()
    print(f"MyService notification sent successfully (HTTP {response.status_code})")
    sys.exit(0)
except Exception as e:
    print(f"MyService notification failed: {str(e)}", file=sys.stderr)
    sys.exit(1)
```

### Available Environment Variables

| Variable | Description |
|----------|-------------|
| `F2B_IP` | The IP address that was banned/unbanned |
| `F2B_JAIL` | The Fail2Ban jail name |
| `F2B_ACTION` | The action performed (ban/unban) |
| `F2B_TIME` | The time of the event (ISO 8601 format) |
| `F2B_TIMESTAMP` | The Unix timestamp of the event |
| `F2B_COUNTRY` | The country of the IP (if GeoIP is enabled) |
| `F2B_REGION` | The region/state of the IP |
| `F2B_CITY` | The city of the IP |
| `F2B_ISP` | The ISP of the IP |
| `F2B_HOSTNAME` | The hostname of the IP (if available) |
| `F2B_FAILURES` | The number of failures that triggered the ban |

### Creating an HTTP Connector

To create an HTTP connector, add a new connector configuration to your `fail2ban-notify.json` file:

```json
{
  "name": "mywebhook",
  "type": "http",
  "enabled": true,
  "path": "",
  "settings": {
    "url": "https://your-api.com/webhook",
    "header_Content-Type": "application/json",
    "header_Authorization": "Bearer YOUR_TOKEN"
  },
  "timeout": 30,
  "retry_count": 2,
  "retry_delay": 5,
  "description": "Send notifications to MyService API"
}
```

### Registering Your Connector

1. After creating your connector script, make it discoverable:
   ```bash
   sudo fail2ban-notify -discover
   ```

2. Test your connector:
   ```bash
   sudo fail2ban-notify -test myservice
   ```

3. Enable your connector in the configuration:
   ```bash
   sudo nano /etc/fail2ban/fail2ban-notify.json
   ```
   Set `"enabled": true` for your connector.

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.
