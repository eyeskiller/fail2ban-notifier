#!/bin/bash
set -e

# Make connectors executable
chmod +x /etc/fail2ban/connectors/*

# Initialize configuration if it doesn't exist
if [ ! -f /etc/fail2ban/fail2ban-notify.json ]; then
    echo "Initializing fail2ban-notify configuration..."
    /usr/local/bin/fail2ban-notify -init
fi

# Restart fail2ban if it's running
if systemctl is-active --quiet fail2ban; then
    echo "Restarting fail2ban service..."
    systemctl restart fail2ban
else
    echo "fail2ban service is not running. Please start it manually:"
    echo "  sudo systemctl start fail2ban"
fi

# Print success message
echo "fail2ban-notify has been installed successfully!"
echo "To configure notification services, edit /etc/fail2ban/fail2ban-notify.json"
echo "To add the notification action to a jail, edit /etc/fail2ban/jail.local and add 'notify' to the action line."
echo "Example:"
echo "  action = iptables[name=SSH, port=ssh, protocol=tcp]"
echo "           notify"
echo ""
echo "For more information, visit: https://github.com/eyeskiller/fail2ban-notifier-go"

exit 0
