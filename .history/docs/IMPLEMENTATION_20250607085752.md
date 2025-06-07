# Implementation Guide - Step by Step

## 🚀 From Zero to Production in 30 Minutes

This guide will take you from an empty repository to a fully functional, production-ready fail2ban notification system with automated releases.

## 📋 Prerequisites

- [ ] GitHub account
- [ ] Go 1.19+ installed
- [ ] Git configured
- [ ] Docker installed (optional)
- [ ] Linux server with fail2ban (for testing)

## 🏗️ Phase 1: Repository Setup (5 minutes)

### Step 1: Create Repository

```bash
# 1. Create new repository on GitHub
# Repository name: fail2ban-notify-go
# Description: Modular notification system for Fail2Ban
# Add README, .gitignore (Go), License (MIT)

# 2. Clone locally
git clone https://github.com/eyeskiller/fail2ban-notifier.git
cd fail2ban-notify-go
```

### Step 2: Initialize Project Structure

```bash
# Create directory structure
mkdir -p {cmd/fail2ban-notify,internal/{config,connectors,geoip,version},pkg/types,connectors,configs,scripts,build/package/{deb,rpm,docker},docs,tests/{unit,integration,fixtures},.github/workflows}

# Initialize Go module
go mod init github.com/eyeskiller/fail2ban-notifier

# Create VERSION file
echo "1.0.0" > VERSION

# Create basic .gitignore
cat > .gitignore << 'EOF'
# Binaries
*.exe
*.dll
*.so
*.dylib
*.test
*.out
coverage.html

# Build
dist/
build/
bin/
vendor/

# IDE
.vscode/
.idea/
*.swp

# OS
.DS_Store
Thumbs.db

# Config files with secrets
config.local.json
.env*

# Logs
*.log

# GoReleaser
goreleaser.yml.backup
EOF
```

## 💻 Phase 2: Core Implementation (10 minutes)

### Step 3: Copy Core Files

Copy all the artifact files we created:

1. **Main Application** → `cmd/fail2ban-notify/main.go`
2. **Connector Scripts** → `connectors/`
3. **Fail2ban Config** → `configs/notify.conf`
4. **Build System** → `Makefile`, `.goreleaser.yml`
5. **GitHub Actions** → `.github/workflows/`
6. **Scripts** → `scripts/`
7. **Documentation** → `README.md`, `docs/`

### Step 4: Create Version Package

```bash
# Create internal/version/version.go
cat > internal/version/version.go << 'EOF'
package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
	GoVersion = "unknown"
)

func GetVersion() string {
	return Version
}

func GetBuildInfo() string {
	return fmt.Sprintf(
		"Version: %s\nBuild Time: %s\nGit Commit: %s\nGo Version: %s\nOS/Arch: %s/%s",
		Version, BuildTime, GitCommit, GoVersion, runtime.GOOS, runtime.GOARCH,
	)
}
EOF
```

### Step 5: Update Main Application

Add version information to main.go:

```go
// Add to imports
import "github.com/eyeskiller/fail2ban-notifier/internal/version"

// Add version flag handling
if *version {
    fmt.Println(version.GetBuildInfo())
    return
}
```

### Step 6: First Build and Test

```bash
# Install development tools
make dev-setup

# First build
make build

# Run tests (create basic test first)
mkdir -p tests/unit
cat > tests/unit/version_test.go << 'EOF'
package unit

import (
	"testing"
	"github.com/eyeskiller/fail2ban-notifier/internal/version"
)

func TestGetVersion(t *testing.T) {
	v := version.GetVersion()
	if v == "" {
		t.Error("Version should not be empty")
	}
}
EOF

make test
```

## ⚙️ Phase 3: CI/CD Setup (5 minutes)

### Step 7: GitHub Actions Configuration

```bash
# Copy workflow files to .github/workflows/
# These were created in the previous artifacts:
# - ci.yml
# - release.yml  
# - nightly.yml
# - dependency-update.yml

# Update repository references in all files
find .github -name "*.yml" -exec sed -i 's/YOUR_USERNAME/your-actual-username/g' {} \;
find . -name "*.md" -exec sed -i 's/YOUR_USERNAME/your-actual-username/g' {} \;
find . -name "*.go" -exec sed -i 's/YOUR_USERNAME/your-actual-username/g' {} \;
```

### Step 8: Enable GitHub Features

1. **Go to Repository Settings**
2. **Actions tab** → Enable GitHub Actions
3. **Pages tab** → Enable GitHub Pages (for documentation)
4. **Security tab** → Enable Dependabot alerts
5. **Branches tab** → Add branch protection rules for `main`

### Step 9: First Commit and Test

```bash
# Add all files
git add .

# Initial commit
git commit -m "feat: initial implementation of modular fail2ban notification system

- Core Go application with modular connector architecture
- Built-in connectors for Discord, Teams, Slack, Telegram, Email
- Automated build system with cross-platform support
- Complete CI/CD pipeline with GitHub Actions
- Package management for multiple Linux distributions
- Docker containerization support
- Comprehensive documentation and testing"

# Push to trigger CI
git push origin main

# Check GitHub Actions tab to see CI running
```

## 🔌 Phase 4: Connector Testing (5 minutes)

### Step 10: Test Connectors Locally

```bash
# Install locally for testing
make install

# Test basic functionality
sudo fail2ban-notify -version
sudo fail2ban-notify -init
sudo fail2ban-notify -discover

# Test with fake data
sudo fail2ban-notify -ip="192.168.1.100" -jail="test" -action="ban" -debug

# Test individual connectors
sudo scripts/test-connector.sh
```

### Step 11: Configure Real Services

```bash
# Edit configuration
sudo nano /etc/fail2ban/fail2ban-notify.json

# Enable Discord (example)
{
  "connectors": [
    {
      "name": "discord",
      "type": "script",
      "enabled": true,
      "path": "/etc/fail2ban/connectors/discord.sh",
      "settings": {
        "DISCORD_WEBHOOK_URL": "YOUR_ACTUAL_WEBHOOK_URL"
      }
    }
  ]
}

# Test with real webhook
sudo fail2ban-notify -ip="1.2.3.4" -jail="sshd" -action="ban" -debug
```

## 🚀 Phase 5: First Release (5 minutes)

### Step 12: Create First Release

```bash
# Ensure everything is committed
git add .
git commit -m "chore: prepare for v1.0.0 release"
git push origin main

# Create and push tag (this triggers release)
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions will automatically:
# ✅ Run full test suite
# ✅ Build binaries for all platforms  
# ✅ Create GitHub release
# ✅ Build and push Docker images
# ✅ Generate installation scripts
```

### Step 13: Verify Release

1. **Check GitHub Actions** - All workflows should pass
2. **Check Releases Page** - Release should be created with binaries
3. **Check Packages** - Docker images should be available
4. **Test Installation** - Try the installation script

```bash
# Test installation script on clean system
curl -fsSL https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/main/scripts/install.sh | sudo bash
```

## 🌟 Phase 6: Production Deployment

### Step 14: Deploy to Production Server

```bash
# On your production server with fail2ban
curl -fsSL https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/main/scripts/install.sh | sudo bash

# Configure your services
sudo nano /etc/fail2ban/fail2ban-notify.json

# Add to jail configuration
sudo nano /etc/fail2ban/jail.local
# Add: action = iptables[...] notify

# Restart fail2ban
sudo systemctl restart fail2ban
sudo systemctl status fail2ban
```

### Step 15: Monitor and Validate

```bash
# Monitor fail2ban logs
sudo tail -f /var/log/fail2ban.log

# Test by triggering a ban (be careful!)
# Or manually test
sudo fail2ban-notify -ip="malicious.ip.address" -jail="sshd" -action="ban"

# Check that notifications are received in your configured services
```

## 📈 Phase 7: Community & Maintenance

### Step 16: Documentation and Community

```bash
# Update README with your specifics
# Add examples and screenshots
# Create CONTRIBUTING.md
# Set up GitHub Discussions
# Create issue templates

# Add badges to README
[![Build Status](https://github.com/eyeskiller/fail2ban-notifier/workflows/CI/badge.svg)](https://github.com/eyeskiller/fail2ban-notifier/actions)
[![Release](https://img.shields.io/github/release/eyeskiller/fail2ban-notifier.svg)](https://github.com/eyeskiller/fail2ban-notifier/releases)
[![Docker Pulls](https://img.shields.io/docker/pulls/YOUR_USERNAME/fail2ban-notify)](https://github.com/eyeskiller/fail2ban-notifier/pkgs/container/fail2ban-notify)
```

### Step 17: Enable Advanced Features

```bash
# Set up advanced monitoring
# Enable automatic dependency updates
# Configure security scanning
# Set up performance monitoring
# Create usage analytics (opt-in)

# Enable GitHub features
# - Branch protection
# - Require PR reviews
# - Require status checks
# - Enable merge queue
```

## 🎯 Success Metrics

After completing this guide, you should have:

✅ **Fully functional notification system**
- All connectors working
- Real notifications being sent
- Fail2ban integration active

✅ **Professional development workflow**
- Automated testing and building
- Code quality checks
- Security scanning

✅ **Production-ready releases**
- Multi-platform binaries
- Package manager support
- Docker images
- Automated installation

✅ **Community-ready project**
- Comprehensive documentation
- Issue templates
- Contributing guidelines
- Professional presentation

## 🚨 Troubleshooting Common Issues

### Build Issues

```bash
# Go module issues
go clean -modcache
go mod download
go mod tidy

# Permission issues
sudo chown -R $USER:$USER .
chmod +x scripts/*.sh
```

### GitHub Actions Issues

```bash
# Check workflow logs in GitHub Actions tab
# Common issues:
# - Repository secrets not set
# - Branch protection conflicts  
# - Workflow permissions

# Fix permissions in .github/workflows/*.yml
permissions:
  contents: write
  packages: write
```

### Connector Issues

```bash
# Test individual connectors
sudo /etc/fail2ban/connectors/discord.sh
echo $?  # Should be 0 for success

# Check environment variables
env | grep F2B_
env | grep DISCORD_

# Validate webhook URLs
curl -X POST -H "Content-Type: application/json" -d '{"test":"data"}' YOUR_WEBHOOK_URL
```

### Installation Issues

```bash
# Check fail2ban status
sudo systemctl status fail2ban

# Verify action configuration
sudo fail2ban-client -t

# Check file permissions
ls -la /usr/local/bin/fail2ban-notify
ls -la /etc/fail2ban/connectors/
```

## 🎉 Congratulations!

You now have a fully functional, production-ready, modular fail2ban notification system with:

- **Professional CI/CD pipeline**
- **Multi-platform support**
- **Automated releases**
- **Comprehensive testing**
- **Community-ready documentation**
- **Enterprise-grade security**

Your system is ready for production use and community contributions!
