#!/bin/bash
# scripts/build-rpm.sh - Build RPM package

set -euo pipefail

VERSION=${1:-"1.0.0"}
ARCH=${2:-"x86_64"}
PACKAGE_NAME="fail2ban-notify"
BUILD_DIR="build/package/rpm"
SPEC_FILE="$BUILD_DIR/$PACKAGE_NAME.spec"

echo "🏗️ Building RPM package v$VERSION for $ARCH..."

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

# Create spec file
cat > "$SPEC_FILE" << EOF
Name:           $PACKAGE_NAME
Version:        $VERSION
Release:        1%{?dist}
Summary:        Modular notification system for Fail2Ban
License:        MIT
URL:            https://github.com/eyeskiller/fail2ban-notifier
Source0:        %{name}-%{version}.tar.gz
BuildArch:      $ARCH

Requires:       fail2ban, curl
Recommends:     python3

%description
A modern, modular notification system for Fail2Ban written in Go.
Send real-time security alerts to Discord, Microsoft Teams, Slack,
Telegram, email, or any custom service through pluggable connectors.

%prep
%setup -q

%install
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/etc/fail2ban/{action.d,connectors}
mkdir -p %{buildroot}/usr/share/doc/%{name}

# Install binary
install -m 755 dist/$PACKAGE_NAME-linux-$ARCH %{buildroot}/usr/local/bin/$PACKAGE_NAME

# Install configurations
install -m 644 configs/notify.conf %{buildroot}/etc/fail2ban/action.d/
install -m 755 connectors/*.sh %{buildroot}/etc/fail2ban/connectors/ 2>/dev/null || true
install -m 755 connectors/*.py %{buildroot}/etc/fail2ban/connectors/ 2>/dev/null || true

# Install scripts
install -m 755 scripts/create-connector.sh %{buildroot}/usr/local/bin/

# Install documentation
install -m 644 README.md %{buildroot}/usr/share/doc/%{name}/
install -m 644 LICENSE %{buildroot}/usr/share/doc/%{name}/ 2>/dev/null || true

%post
# Initialize configuration if it doesn't exist
if [ ! -f /etc/fail2ban/fail2ban-notify.json ]; then
    echo "Initializing fail2ban-notify configuration..."
    /usr/local/bin/fail2ban-notify -init
fi

# Restart fail2ban if it's running
if systemctl is-active --quiet fail2ban; then
    echo "Restarting fail2ban service..."
    systemctl restart fail2ban
fi

echo "fail2ban-notify installed successfully!"

%preun
# Stop fail2ban before removal
if systemctl is-active --quiet fail2ban; then
    systemctl stop fail2ban
fi

%postun
if [ \$1 -eq 0 ]; then
    # Package is being removed, not upgraded
    systemctl start fail2ban 2>/dev/null || true
fi

%files
%defattr(-,root,root,-)
/usr/local/bin/$PACKAGE_NAME
/usr/local/bin/create-connector.sh
%config(noreplace) /etc/fail2ban/action.d/notify.conf
/etc/fail2ban/connectors/*
%doc /usr/share/doc/%{name}/*

%changelog
* $(date '+%a %b %d %Y') Your Name <your-email@example.com> - $VERSION-1
- Initial package release
EOF

# Build RPM
rpmbuild -ba "$SPEC_FILE" --define "_topdir $(pwd)/$BUILD_DIR"

echo "✅ RPM package created in: $BUILD_DIR/RPMS/$ARCH/"
