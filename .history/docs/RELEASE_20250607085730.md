# Release Guide & Checklist

## 🚀 Quick Start Guide

### 1. First Time Setup

```bash
# Clone and setup
git clone https://github.com/eyeskiller/fail2ban-notifier.git
cd fail2ban-notify-go

# Setup development environment
make dev-setup

# Initial build and test
make all

# Install locally for testing
make install
```

### 2. Daily Development Workflow

```bash
# Start new feature
git checkout -b feature/my-new-feature

# Development cycle
make test          # Run tests
make lint          # Check code quality
make build         # Build binary
make install       # Install locally
sudo fail2ban-notify -ip=1.2.3.4 -jail=test -debug  # Test

# Commit and push
git add .
git commit -m "feat: add amazing new feature"
git push origin feature/my-new-feature
```

### 3. Creating a Release

```bash
# 1. Prepare release
git checkout main
git pull origin main

# 2. Update version
echo "1.2.0" > VERSION
make docs  # Update documentation

# 3. Update changelog
cat >> CHANGELOG.md << 'EOF'
## [1.2.0] - 2024-01-15

### Added
- New connector for XYZ service
- Enhanced error handling

### Fixed
- Bug in geolocation lookup
- Memory leak in connector execution

### Changed
- Improved performance by 20%
EOF

# 4. Commit version bump
git add VERSION CHANGELOG.md docs/
git commit -m "chore: bump version to 1.2.0"
git push origin main

# 5. Create and push tag
git tag v1.2.0
git push origin v1.2.0

# 🎉 GitHub Actions will automatically:
# - Run full test suite
# - Build for all platforms
# - Create GitHub release with binaries
# - Build and push Docker images
# - Update package managers (brew, apt, etc.)
```

## 📋 Release Checklist

### Pre-Release Checklist

- [ ] **Code Quality**
  - [ ] All tests passing (`make test`)
  - [ ] Linter checks passed (`make lint`)
  - [ ] Security scan passed (`make security`)
  - [ ] Code coverage > 80%

- [ ] **Documentation**
  - [ ] README.md updated
  - [ ] CHANGELOG.md updated with new version
  - [ ] API documentation generated (`make docs`)
  - [ ] Installation instructions tested

- [ ] **Testing**
  - [ ] Unit tests pass
  - [ ] Integration tests pass
  - [ ] Manual testing completed
  - [ ] All connectors tested (`scripts/test-connector.sh`)

- [ ] **Version Management**
  - [ ] VERSION file updated
  - [ ] Version consistent across all files
  - [ ] Git tag follows semantic versioning (v1.2.3)

### Release Process

- [ ] **Preparation**
  - [ ] Main branch is stable
  - [ ] All PRs merged
  - [ ] Version bumped and committed
  - [ ] Tag created and pushed

- [ ] **Automated Release** (GitHub Actions)
  - [ ] CI/CD pipeline passed
  - [ ] Binaries built for all platforms
  - [ ] Docker images pushed
  - [ ] GitHub release created
  - [ ] Package managers updated

- [ ] **Post-Release**
  - [ ] Release announcement posted
  - [ ] Documentation sites updated
  - [ ] Social media updates
  - [ ] Monitor for issues

### Hotfix Process

For critical bugs requiring immediate release:

```bash
# 1. Create hotfix branch from main
git checkout main
git checkout -b hotfix/critical-fix

# 2. Fix the issue
# ... make minimal changes ...

# 3. Test thoroughly
make test
make lint

# 4. Bump patch version
echo "1.2.1" > VERSION
git add VERSION
git commit -m "fix: critical security vulnerability"

# 5. Create tag and push
git tag v1.2.1
git push origin hotfix/critical-fix
git push origin v1.2.1

# 6. Merge back to main
git checkout main
git merge hotfix/critical-fix
git push origin main
```

## 🔧 Build System Details

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make dev-setup` | Install development tools |
| `make build` | Build for current platform |
| `make build-all` | Cross-compile for all platforms |
| `make test` | Run unit tests |
| `make test-integration` | Run integration tests |
| `make lint` | Run code linting |
| `make install` | Install locally |
| `make clean` | Clean build artifacts |
| `make release TAG=v1.0.0` | Create tagged release |
| `make snapshot` | Create snapshot release |
| `make docker-build` | Build Docker image |
| `make package-deb` | Create Debian package |
| `make package-rpm` | Create RPM package |

### GoReleaser Configuration

The `.goreleaser.yml` file handles:
- ✅ Cross-platform builds (Linux, macOS, Windows)
- ✅ Archive creation with proper naming
- ✅ GitHub release with changelog
- ✅ Docker image builds and pushes
- ✅ Package manager updates (Homebrew)
- ✅ Debian/RPM package creation

### GitHub Actions Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Push/PR | Test, lint, build |
| `release.yml` | Tag push | Create releases |
| `nightly.yml` | Schedule | Nightly builds |
| `dependency-update.yml` | Schedule | Update deps |

## 📦 Package Management

### Homebrew

```bash
# Install via Homebrew (macOS/Linux)
brew tap YOUR_USERNAME/tap
brew install fail2ban-notify

# Update formula (automated via GitHub Actions)
brew update
brew upgrade fail2ban-notify
```

### Debian/Ubuntu

```bash
# Install .deb package
wget https://github.com/eyeskiller/fail2ban-notifier/releases/download/v1.0.0/fail2ban-notify_1.0.0_amd64.deb
sudo dpkg -i fail2ban-notify_1.0.0_amd64.deb

# Or add APT repository (future enhancement)
```

### Red Hat/CentOS

```bash
# Install .rpm package
wget https://github.com/eyeskiller/fail2ban-notifier/releases/download/v1.0.0/fail2ban-notify-1.0.0-1.x86_64.rpm
sudo rpm -i fail2ban-notify-1.0.0-1.x86_64.rpm

# Or add YUM repository (
