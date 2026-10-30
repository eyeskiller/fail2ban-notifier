# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.1.3] - 2026-10-30

### Added
- Persistent GeoIP cache with file locking for better performance
- Multiple GeoIP providers (GeoLite2, ipapi, ipgeolocation, ipwhois) with rate limiting
- HTTP client with improved error handling

### Fixed
- Fixed `notify-enhanced` action placeholders (`<n>` → `<name>`)
- Fixed `<debug_flag>` not expanding correctly in fail2ban actions
- Removed unsupported actionstart/actionstop/actionflush calls
- Fixed broken `[Filter]` section in notify-enhanced.conf

### Changed
- Switched CI/CD to GitHub-hosted runners (free for public repos)
- Added Snyk policy to ignore gopls false positive CVE-2026-42503

### Security
- Hardened GeoIP providers with rate limiting and error handling
- Added persistent cache with file locking

---

## [1.1.2] - 2026-07-06

### Fixed
- **Duplicate connectors in config**: `-init` no longer adds auto-discovered connectors when a sample connector with the same name already exists. Previously, running `-init` would add both the sample `discord` (from `CreateSampleConfig()`) and the auto-discovered `discord` (from the connectors directory), causing the setup wizard and status command to show inconsistent enabled/disabled states.

---

## [1.1.1] - 2026-07-06

### Added
- **Interactive Setup Wizard**: Run `sudo fail2ban-notify -setup` for a fully interactive setup
- **PagerDuty Connector**: New `connectors/pagerduty.sh` script for incident management
- Expanded test coverage (26% → 47% overall)

### Changed
- `install.sh` rewritten to download pre-built binaries (no Go toolchain needed)
- README badges updated, install instructions streamlined
- Go pinned to `1.26.4` in CI/CD for deterministic builds
- All gosec findings suppressed with `#nosec` annotations (0 issues)

---

## [1.1.0] - 2026-07-06

### Added
- **GoReleaser build pipeline**: Automated builds for linux/amd64, arm64, 386, and armv7
- **GitHub Actions CI**: Automated testing, govulncheck, and gosec security scanning
- **Self-hosted runner support**: CI migrated to self-hosted runners
- **Pre-built binary install**: install.sh now downloads binaries from GitHub Releases
- **Unit tests**: 7 new test suites across config, connectors, and geoip packages
- **Email connector**: New `email.py` with SMTP and HTML template support

### Fixed
- **Body.Close() error propagation**: Fixed defer/err shadowing bug in IPAPIService and IPGeolocationService
- **Linter warnings**: Removed unused parameters, simplified closures, fixed empty error handling
- **Shell script hardening**: Added URL validation and improved error handling

### Security
- **Go 1.26.4 upgrade**: Resolves 20+ Go stdlib vulnerabilities
- **gosec audit fixes**: Critical correctness and security fixes

---

## [1.0.0] - 2025-06-07

First stable release of Fail2Ban Notifier.

### Features
- Native support for Slack, Microsoft Teams, Discord, Telegram, and Email
- Custom connector support (run your own scripts)
- Simple config via environment variables
- Designed for multi-server environments
- Instant notifications without setting up a full monitoring stack

---

[v1.1.3]: https://github.com/eyeskiller/fail2ban-notifier/releases/tag/v1.1.3
[1.1.2]: https://github.com/eyeskiller/fail2ban-notifier/releases/tag/1.1.2
[1.1.1]: https://github.com/eyeskiller/fail2ban-notifier/releases/tag/1.1.1
[1.1.0]: https://github.com/eyeskiller/fail2ban-notifier/releases/tag/1.1.0
[1.0.0]: https://github.com/eyeskiller/fail2ban-notifier/releases/tag/1.0.0
