# fail2ban-notifier

A notification system for Fail2Ban that sends alerts to various services when IPs are banned or unbanned.

## Features

- **Multiple Notification Services**: Support for Discord, Slack, Microsoft Teams, Telegram, Email, and custom webhooks
- **GeoIP Integration**: Automatically lookup and include geographic information about banned IPs
- **Flexible Configuration**: Easy to configure and extend with new notification services
- **Fail2Ban Integration**: Seamlessly integrates with Fail2Ban's action system
- **Customizable Templates**: Notification messages can be customized for each service
- **Retry Mechanism**: Built-in retry for failed notifications
- **Connector Discovery**: Automatically discovers available notification connectors

## Installation

### One-liner Installation

```bash
curl -sSL https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/master/install.sh | sudo bash
```

### Manual Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/eyeskiller/fail2ban-notifier.git
   cd fail2ban-notifier
   ```

2. Build and install:
   ```bash
   make build
   sudo make install
   ```

## Configuration

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

### Enabling Connectors

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

### Fail2Ban Integration

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

## Usage

### Command Line Options

```
Usage of fail2ban-notify:
  -action string
        Action performed (ban/unban) (default "ban")
  -config string
        Path to configuration file (default "/etc/fail2ban/fail2ban-notify.json")
  -debug
        Enable debug logging
  -discover
        Discover available connectors
  -failures int
        Number of failures
  -init
        Initialize configuration file
  -ip string
        IP address that was banned/unbanned
  -jail string
        Fail2ban jail name
  -status
        Show connector status
  -test string
        Test specific connector
  -version
        Show version information
```

### Examples

Test a connector:
```bash
sudo fail2ban-notify -test discord
```

Check connector status:
```bash
sudo fail2ban-notify -status
```

Manually trigger a notification:
```bash
sudo fail2ban-notify -ip="192.168.1.100" -jail="ssh" -action="ban" -failures=5
```

## Supported Notification Services

- **Discord**: Send notifications to Discord channels via webhooks
- **Slack**: Send notifications to Slack channels via webhooks
- **Microsoft Teams**: Send notifications to Teams channels via webhooks
- **Telegram**: Send notifications to Telegram chats via bot API
- **Email**: Send email notifications via SMTP
- **Custom Webhook**: Send notifications to any HTTP endpoint

## License

This project is licensed under the MIT License - see the LICENSE file for details.
