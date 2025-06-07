# Accessing Build Artifacts on Self-Hosted Runner

This document explains how to access build artifacts that are stored locally on the self-hosted runner.

## Artifact Storage Location

All artifacts are stored in the `$HOME/runner_artifacts/` directory on the self-hosted runner. The directory structure is as follows:

```
$HOME/runner_artifacts/
├── coverage/
│   ├── coverage-{SHA}.html
│   └── coverage-norace-{SHA}.out
├── binaries/
│   └── {SHA}/
│       ├── fail2ban-notify-linux-amd64
│       ├── fail2ban-notify-linux-arm64
│       ├── fail2ban-notify-darwin-amd64
│       ├── fail2ban-notify-darwin-arm64
│       └── fail2ban-notify-windows-amd64.exe
└── nightly/
    └── YYYY-MM-DD/
        └── (nightly build artifacts)
```

Where:
- `{SHA}` is the Git commit SHA
- `YYYY-MM-DD` is the date of the nightly build

## Accessing Artifacts

To access the artifacts, you need SSH access to the self-hosted runner. Once connected, you can navigate to the artifact directories:

```bash
# List all coverage reports
ls -la $HOME/runner_artifacts/coverage/

# List all binary builds
ls -la $HOME/runner_artifacts/binaries/

# List all nightly builds
ls -la $HOME/runner_artifacts/nightly/
```

## Downloading Artifacts

You can download artifacts from the self-hosted runner using SCP or SFTP:

```bash
# Example SCP command to download binaries for a specific commit
scp -r user@self-hosted-runner:$HOME/runner_artifacts/binaries/{SHA}/* ./local-directory/

# Example SCP command to download the latest nightly build
scp -r user@self-hosted-runner:$HOME/runner_artifacts/nightly/$(date +%Y-%m-%d)/* ./local-directory/
```

## Retention Policy

- Coverage reports: Kept indefinitely
- Binary builds: Kept indefinitely
- Nightly builds: Kept for 7 days

## Advantages Over GitHub Artifact Storage

1. No storage quota limitations
2. Faster access for subsequent jobs in the same workflow
3. Persistent storage between workflow runs
4. No need to re-upload artifacts for different jobs

## Maintenance

If disk space becomes a concern, you can manually clean up old artifacts:

```bash
# Remove old binary builds (example: older than 30 days)
find $HOME/runner_artifacts/binaries -type d -mtime +30 -exec rm -rf {} \; 2>/dev/null || true

# Remove old coverage reports (example: older than 30 days)
find $HOME/runner_artifacts/coverage -type f -mtime +30 -exec rm -f {} \; 2>/dev/null || true
```
