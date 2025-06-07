#!/usr/bin/env python3
"""
Email Connector for fail2ban-notify
Place this file in /etc/fail2ban/connectors/email.py
"""

import os
import sys
import json
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from datetime import datetime

def get_config():
    """Get configuration from environment variables"""
    return {
        'smtp_server': os.getenv('EMAIL_SMTP_SERVER', 'localhost'),
        'smtp_port': int(os.getenv('EMAIL_SMTP_PORT', '587')),
        'smtp_user': os.getenv('EMAIL_SMTP_USER', ''),
        'smtp_password': os.getenv('EMAIL_SMTP_PASSWORD', ''),
        'smtp_tls': os.getenv('EMAIL_SMTP_TLS', 'true').lower() == 'true',
        'from_email': os.getenv('EMAIL_FROM', 'fail2ban@localhost'),
        'to_email': os.getenv('EMAIL_TO', 'admin@localhost'),
        'subject_prefix': os.getenv('EMAIL_SUBJECT_PREFIX', '[Fail2Ban]'),
    }

def get_notification_data():
    """Get notification data from environment variables and stdin"""
    data = {
        'ip': os.getenv('F2B_IP', 'unknown'),
        'jail': os.getenv('F2B_JAIL', 'unknown'),
        'action': os.getenv('F2B_ACTION', 'ban'),
        'time': os.getenv('F2B_TIME', datetime.now().isoformat()),
        'country': os.getenv('F2B_COUNTRY', ''),
        'region': os.getenv('F2B_REGION', ''),
        'city': os.getenv('F2B_CITY', ''),
        'isp': os.getenv('F2B_ISP', ''),
        'hostname': os.getenv('F2B_HOSTNAME', ''),
        'failures': int(os.getenv('F2B_FAILURES', '0')),
    }
    
    # Try to read JSON from stdin as well
    try:
        if not sys.stdin.isatty():
            json_data = json.loads(sys.stdin.read())
            data.update(json_data)
    except (json.JSONDecodeError, Exception):
        pass
    
    return data

def create_email_content(data, config):
    """Create email subject and body"""
    action = data['action'].capitalize()
    emoji = "🚫" if data['action'] == 'ban' else "✅"
    
    subject = f"{config['subject_prefix']} {emoji} {action}: {data['ip']} in {data['jail']}"
    
    # Build location string
    location = ""
    if data['country']:
        location = f" from {data['country']}"
        if data['city']:
            location = f" from {data['city']}, {data['country']}"
    
    # Create HTML body
    html_body = f"""
    <html>
    <head>
        <style>
            body {{ font-family: Arial, sans-serif; margin: 20px; }}
            .header {{ background-color: {'#ffebee' if data['action'] == 'ban' else '#e8f5e8'}; 
                      padding: 15px; border-radius: 5px; margin-bottom: 20px; }}
            .info-table {{ border-collapse: collapse; width: 100%; }}
            .info-table td {{ border: 1px solid #ddd; padding: 8px; }}
            .info-table th {{ border: 1px solid #ddd; padding: 8px; background-color: #f2f2f2; }}
            .highlight {{ font-weight: bold; color: {'#d32f2f' if data['action'] == 'ban' else '#388e3c'}; }}
        </style>
    </head>
    <body>
        <div class="header">
            <h2>{emoji} Fail2Ban {action} Alert</h2>
            <p>IP <span class="highlight">{data['ip']}</span>{location} has been <strong>{data['action']}ned</strong> in jail '<strong>{data['jail']}</strong>'</p>
        </div>
        
        <table class="info-table">
            <tr><th>Field</th><th>Value</th></tr>
            <tr><td>IP Address</td><td>{data['ip']}</td></tr>
            <tr><td>Jail</td><td>{data['jail']}</td></tr>
            <tr><td>Action</td><td>{action}</td></tr>
            <tr><td>Time</td><td>{data['time']}</td></tr>
    """
    
    if data['failures'] > 0:
        html_body += f"<tr><td>Failures</td><td>{data['failures']}</td></tr>"
    
    if data['country']:
        location_str = data['city'] + ", " + data['country'] if data['city'] else data['country']
        html_body += f"<tr><td>Location</td><td>{location_str}</td></tr>"
    
    if data['isp']:
        html_body += f"<tr><td>ISP</td><td>{data['isp']}</td></tr>"
    
    if data['hostname']:
        html_body += f"<tr><td>Hostname</td><td>{data['hostname']}</td></tr>"
    
    html_body += """
        </table>
        
        <p style="margin-top: 20px; font-size: 12px; color: #666;">
            This is an automated security alert from Fail2Ban.<br>
            For more information about this IP, visit: 
            <a href="https://whatismyipaddress.com/ip/{ip}">whatismyipaddress.com/ip/{ip}</a>
        </p>
    </body>
    </html>
    """.format(ip=data['ip'])
    
    # Create plain text version
    text_body = f"""
Fail2Ban {action} Alert

IP {data['ip']}{location} has been {data['action']}ned in jail '{data['jail']}'

Details:
- IP Address: {data['ip']}
- Jail: {data['jail']}
- Action: {action}
- Time: {data['time']}
"""
    
    if data['failures'] > 0:
        text_body += f"- Failures: {data['failures']}\n"
    
    if data['country']:
        location_str = data['city'] + ", " + data['country'] if data['city'] else data['country']
        text_body += f"- Location: {location_str}\n"
    
    if data['isp']:
        text_body += f"- ISP: {data['isp']}\n"
    
    if data['hostname']:
        text_body += f"- Hostname: {data['hostname']}\n"
    
    text_body += f"""
For more information about this IP, visit:
https://whatismyipaddress.com/ip/{data['ip']}

This is an automated security alert from Fail2Ban.
"""
    
    return subject, html_body, text_body

def send_email(subject, html_body, text_body, config):
    """Send the email notification"""
    try:
        # Create message
        msg = MIMEMultipart('alternative')
        msg['Subject'] = subject
        msg['From'] = config['from_email']
        msg['To'] = config['to_email']
        
        # Add both plain text and HTML versions
        msg.attach(MIMEText(text_body, 'plain'))
        msg.attach(MIMEText(html_body, 'html'))
        
        # Connect to SMTP server
        server = smtplib.SMTP(config['smtp_server'], config['smtp_port'])
        
        if config['smtp_tls']:
            server.starttls()
        
        if config['smtp_user'] and config['smtp_password']:
            server.login(config['smtp_user'], config['smtp_password'])
        
        # Send email
        server.send_message(msg)
        server.quit()
        
        print(f"Email notification sent successfully to {config['to_email']}")
        return True
        
    except Exception as e:
        print(f"Failed to send email: {e}", file=sys.stderr)
        return False

def main():
    """Main function"""
    config = get_config()
    
    # Validate required configuration
    if not config['to_email'] or config['to_email'] == 'admin@localhost':
        print("Error: EMAIL_TO not configured", file=sys.stderr)
        sys.exit(1)
    
    # Get notification data
    data = get_notification_data()
    
    # Create email content
    subject, html_body, text_body = create_email_content(data, config)
    
    # Send email
    if send_email(subject, html_body, text_body, config):
        sys.exit(0)
    else:
        sys.exit(1)

if __name__ == '__main__':
    main()