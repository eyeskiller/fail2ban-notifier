# fail2ban-notifier

<div align="center">
  <img src="https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/master/assets/logo.png" alt="fail2ban-notifier logo" width="200" height="200" onerror="this.style.display='none'">

  <h3>A powerful notification system for <a href="https://github.com/fail2ban/fail2ban">Fail2Ban</a></h3>
  <p>Send alerts to various services when IPs are banned or unbanned</p>

  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
  [![Go Version](https://img.shields.io/github/go-mod/go-version/eyeskiller/fail2ban-notifier)](https://go.dev)
  [![Release](https://img.shields.io/github/v/release/eyeskiller/fail2ban-notifier)](https://github.com/eyeskiller/fail2ban-notifier/releases)
  [![Tests](https://img.shields.io/github/actions/workflow/status/eyeskiller/fail2ban-notifier/ci.yml?branch=main&label=tests)](https://github.com/eyeskiller/fail2ban-notifier/actions)
  [![GitHub stars](https://img.shields.io/github/stars/eyeskiller/fail2ban-notifier?style=social)](https://github.com/eyeskiller/fail2ban-notifier/stargazers)
</div>

## ✨ Features

- **🔔 Multiple Notification Services**: Support for Discord, Slack, Microsoft Teams, Telegram, Email, PagerDuty, and custom webhooks
- **🌎 GeoIP Integration**: HTTPS ipwho.is lookups by default, optional offline MaxMind GeoLite2, legacy ip-api.com behind explicit consent, and persistent cross-process caching
- **⚙️ Flexible Configuration**: Easy to configure and extend with new notification services
- **🔒 Fail2Ban Integration**: Seamlessly integrates with Fail2Ban's action system
- **📝 Customizable Templates**: Notification messages can be customized for each service
- **🔄 Retry Mechanism**: Built-in retry for failed notifications
- **🔍 Connector Discovery**: Automatically discovers available notification connectors
- **🧩 Extensible**: Create your own custom connectors to integrate with any service

## 📥 Installation

### One-line Installation (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/refs/heads/main/install.sh | sudo bash
```

This installation script will:
1. Fetch the latest release tag from GitHub
2. Detect your system architecture (amd64, arm64, 386, armv7)
3. Download the pre-built binary archive from GitHub Releases
4. Install the binary, configuration files, and connector scripts
5. Initialize the configuration at `/etc/fail2ban/fail2ban-notify.json`

The installation process will:
- Install the binary to `/usr/local/bin/fail2ban-notify`
- Copy configuration files to `/etc/fail2ban/action.d/`
- Copy connector scripts to `/etc/fail2ban/connectors/`
- Initialize the configuration at `/etc/fail2ban/fail2ban-notify.json`
- Create the persistent GeoIP cache directory at `/var/cache/fail2ban-notifier`

> **Note**: Only Linux is supported for pre-built binaries. On other systems, build from source using `make build && sudo make install`.

## 🚀 Quick Start (Recommended)

After installation, run the interactive setup wizard to configure everything in minutes:

```bash
sudo fail2ban-notify -setup
```

The wizard will guide you through:
1. Checking prerequisites (Fail2Ban installation, permissions)
2. Installing the Fail2Ban action configuration (`notify.conf`)
3. Creating the application configuration
4. Selecting a GeoIP provider (ipwho.is, GeoLite2, or explicitly opting in to legacy ip-api.com)
5. Selecting and configuring notification connectors (Discord, Slack, Telegram, PagerDuty, etc.)
6. Testing your connectors with sample data
7. Adding the notify action to your Fail2Ban jails

All configuration is done interactively — no manual file editing required.

## ⚙️ Configuration (Manual)

If you prefer to configure manually, the configuration file is created at `/etc/fail2ban/fail2ban-notify.json`. You can edit this file to enable and configure your notification services.

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
    "service": "ipwhois",
    "cache": true,
    "ttl": 86400,
    "cache_path": "/var/cache/fail2ban-notifier/geoip.json",
    "allow_insecure_http": false,
    "city_db_path": "/usr/share/GeoIP/GeoLite2-City.mmdb",
    "asn_db_path": "/usr/share/GeoIP/GeoLite2-ASN.mmdb"
  },
  "debug": false,
  "log_level": "info",
  "timeout": 30
}
```

### GeoIP Providers

GeoIP data is optional enrichment. If a provider times out, is rate-limited, has a
missing database, or is disabled, the notification is still delivered with the
original IP address and all non-geographic data. GeoIP is approximate and must
never be used as a trusted signal for authentication or access control.

| Provider | Identifier | Transport | API key | Default | Notes |
|---|---|---|---|---|---|
| ipwho.is | `ipwhois` | HTTPS | No | Yes | About 1,000 requests/day per client IP on the free tier |
| MaxMind GeoLite2 | `geolite2` | None (local MMDB) | No | No | Fully offline after database installation |
| ip-api.com | `ipapi` | HTTP only | No | No | Legacy, explicitly insecure, requires consent |
| ipgeolocation.io | `ipgeolocation` | HTTPS | Yes | No | Existing paid integration is preserved |

#### ipwho.is (recommended)

```json
{
  "geoip": {
    "enabled": true,
    "service": "ipwhois",
    "cache": true,
    "ttl": 86400,
    "cache_path": "/var/cache/fail2ban-notifier/geoip.json"
  }
}
```

ipwho.is is the default for newly generated configurations because it is HTTPS,
does not require an API key, and supports IPv4 and IPv6. The free API is rate
limited; when HTTP 429 is returned, the cooldown is persisted (honoring
`Retry-After` when present) so later Fail2Ban invocations do not keep retrying.

#### MaxMind GeoLite2 (offline)

```json
{
  "geoip": {
    "enabled": true,
    "service": "geolite2",
    "cache": true,
    "ttl": 86400,
    "cache_path": "/var/cache/fail2ban-notifier/geoip.json",
    "city_db_path": "/usr/share/GeoIP/GeoLite2-City.mmdb",
    "asn_db_path": "/usr/share/GeoIP/GeoLite2-ASN.mmdb"
  }
}
```

GeoLite2 performs no network requests during lookups. The City database is
required; the ASN database is optional, and City-only operation continues if it
is missing. The ASN organization is used as a best-effort value for the
notification ISP field; it is not necessarily the actual consumer ISP. The ASN
number is exposed as `asn` / `F2B_ASN`.

Databases are not bundled in this repository or in release binaries. Register
for a free MaxMind account, create a GeoLite2 license key, and install
`geoipupdate`:

```bash
# Example /etc/GeoIP.conf
AccountID YOUR_ACCOUNT_ID
LicenseKey YOUR_LICENSE_KEY
EditionIDs GeoLite2-City GeoLite2-ASN
DatabaseDirectory /usr/share/GeoIP
```

```bash
sudo geoipupdate
```

Run `geoipupdate` periodically (for example from a weekly systemd timer or cron
job) to keep the databases current. Cached results are invalidated automatically
when the database files change. This product includes GeoLite2 data created by
MaxMind, available from [https://www.maxmind.com](https://www.maxmind.com). Use
of GeoLite2 data is subject to MaxMind's license and attribution requirements.

#### ip-api.com (legacy, insecure HTTP)

```json
{
  "geoip": {
    "enabled": true,
    "service": "ipapi",
    "allow_insecure_http": true,
    "cache": true,
    "ttl": 86400
  }
}
```

> **Security warning:** The free ip-api.com endpoint is HTTP-only. Queried IP
> addresses and responses travel unencrypted and can be observed or modified.
> This provider is never selected by default and is never used unless
> `allow_insecure_http` is explicitly set to `true`.

Existing configurations that still contain `"service": "ipapi"` are preserved
after upgrading, but no HTTP requests are made until consent is granted. A
warning is logged and notifications continue without GeoIP enrichment. Migrate
to `ipwhois` (recommended), install `geolite2`, or explicitly set
`"allow_insecure_http": true` to accept the risk. HTTPS is never downgraded to
HTTP automatically.

#### ipgeolocation.io

The existing paid integration remains available by setting
`"service": "ipgeolocation"` and `"api_key": "..."`. The key is sent in the query
string as required by that API and is redacted from error messages.

#### Persistent caching and rate limits

GeoIP results are cached in a JSON file because Fail2Ban starts a short-lived
process for each notification; an in-memory cache cannot survive those
executions. The default location is
`/var/cache/fail2ban-notifier/geoip.json`, with a default TTL of 86400 seconds.

- Entries are keyed by normalized IP address and provider.
- IPv4-mapped IPv6 addresses are normalized before lookup and caching.
- GeoLite2 cache entries include a database fingerprint and are invalidated
  after `geoipupdate`.
- Writes are atomic and protected by an inter-process file lock; the cache file
  is created with mode `0600` and its directory with `0750`.
- Corrupt, expired, invalid, or failed results are never served as successful
  lookups, and failed lookups are never cached.
- Expired entries are pruned and the cache is capped at 10,000 entries.
- Setting `"cache": false` disables persistent caching entirely.
- Rate-limit cooldowns are stored separately, per provider, under the same
  directory.

Fail2Ban usually runs the notifier as root, so the cache directory is created by
the installer as root-owned with mode `0750`. If your deployment runs the action
as a non-root user, grant that user write access to the cache directory or point
`cache_path` at a suitable location.

#### GeoIP troubleshooting

- **`unsupported provider`**: use one of `ipwhois`, `geolite2`, `ipapi`, or
  `ipgeolocation`. Provider names are validated instead of silently replaced;
  an unsupported name is logged as a warning and GeoIP lookups are skipped
  while notifications are still delivered.
- **`allow_insecure_http` warning**: the legacy `ipapi` provider is selected but
  insecure transport has not been authorized. Follow the migration notes above.
- **`rate limited`**: wait for the persisted cooldown to expire, or switch
  providers. Notifications are still delivered meanwhile.
- **GeoLite2 database unavailable**: verify the paths in `city_db_path` and
  `asn_db_path`, file permissions, and run `geoipupdate`. Check the file with
  `mmdblookup`.
- **Cache permission errors**: check ownership and mode of the cache directory
  and file. The notifier logs a warning and continues without caching.
- **Debugging**: run with `-debug` to see GeoIP failures and notification data.
  API keys are not included in GeoIP error messages.

### 🔌 Enabling Connectors

To enable a connector manually:

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

Alternatively, use the setup wizard to configure connectors interactively:
```bash
sudo fail2ban-notify -setup
```

### 🔒 Fail2Ban Integration

To integrate with Fail2Ban manually, add the `notify` action to your jail configuration. The setup wizard (`-setup`) can do this automatically for you.

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
| `-setup` | Run interactive setup wizard | `-setup` |
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

#### Run Setup Wizard
```bash
sudo fail2ban-notify -setup
```
Interactive wizard that walks you through full configuration and deployment.

## 🔔 Supported Notification Services

- **Discord**: Send notifications to Discord channels via webhooks
- **Slack**: Send notifications to Slack channels via webhooks
- **Microsoft Teams**: Send notifications to Teams channels via webhooks
- **Telegram**: Send notifications to Telegram chats via bot API
- **Email**: Send email notifications via SMTP
- **PagerDuty**: Send incidents to PagerDuty via Events API v2 (ban creates a triggered incident, unban resolves it)
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
| `F2B_ISP` | The ISP/ASN organization of the IP (best effort) |
| `F2B_ASN` | The ASN number of the IP (for example `AS15169`) |
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

   Or use the setup wizard to configure and enable it interactively:
   ```bash
   sudo fail2ban-notify -setup
   ```

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.


## ☕ Support My Work

If you like this project and want to keep me fueled with caffeine (because I'm basically a machine that turns coffee into code), consider buying me a coffee:

<a href="https://ko-fi.com/eyeskiller" target="_blank">
  <img src="https://cdn.ko-fi.com/cdn/kofi_button.png" alt="Buy Me a Coffee" height="36" style="vertical-align:middle;">
</a>

Your support helps me maintain and improve this tool. Thanks! 🙌
