#!/bin/bash
# Install development tools for local testing

set -euo pipefail

echo "🛠️ Installing development tools..."

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check Go installation
if ! command -v go &> /dev/null; then
    log_error "Go is not installed. Please install Go 1.19+ first."
    echo "Visit: https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
log_success "Go $GO_VERSION is installed"

# Install golangci-lint
echo "📦 Installing golangci-lint..."
if command -v golangci-lint &> /dev/null; then
    CURRENT_VERSION=$(golangci-lint version 2>/dev/null | head -n1 | awk '{print $4}' || echo "unknown")
    log_warning "golangci-lint already installed (version: $CURRENT_VERSION)"
    echo "Updating to latest version..."
fi

if curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin; then
    log_success "golangci-lint installed/updated"
else
    log_error "Failed to install golangci-lint"
fi

# Install gosec with fallback
echo "🔒 Installing gosec (security scanner)..."
if command -v gosec &> /dev/null; then
    log_warning "gosec already installed"
else
    # Try different package paths for gosec
    GOSEC_PACKAGES=(
        "github.com/securego/gosec/v2/cmd/gosec@latest"
    )

    gosec_installed=false
    for package in "${GOSEC_PACKAGES[@]}"; do
        echo "Trying to install: $package"
        if go install "$package" 2>/dev/null; then
            gosec_installed=true
            log_success "gosec installed successfully"
            break
        fi
    done

    if [[ "$gosec_installed" != "true" ]]; then
        log_warning "Failed to install gosec automatically"
        echo "You can install it manually:"
        echo "  go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"
        echo "Or skip security scanning in tests with --quick flag"
    fi
fi

# Verify installations
echo ""
echo "🔍 Verifying installations..."

# Check Go tools are in PATH
if [[ ":$PATH:" != *":$(go env GOPATH)/bin:"* ]]; then
    log_warning "$(go env GOPATH)/bin is not in PATH"
    echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    echo "export PATH=\"\$(go env GOPATH)/bin:\$PATH\""
fi

# Test installations
tools_working=0
total_tools=0

# Test golangci-lint
((total_tools++))
if command -v golangci-lint &> /dev/null; then
    if golangci-lint version &> /dev/null; then
        VERSION=$(golangci-lint version | head -n1 | awk '{print $4}' || echo "unknown")
        log_success "golangci-lint working (version: $VERSION)"
        ((tools_working++))
    else
        log_error "golangci-lint installed but not working"
    fi
else
    log_error "golangci-lint not found in PATH"
fi

# Test gosec
((total_tools++))
if command -v gosec &> /dev/null; then
    if gosec --version &> /dev/null; then
        VERSION=$(gosec --version 2>/dev/null | head -n1 || echo "unknown")
        log_success "gosec working ($VERSION)"
        ((tools_working++))
    else
        log_error "gosec installed but not working"
    fi
else
    log_warning "gosec not found in PATH (security scanning will be skipped)"
fi

# Summary
echo ""
echo "📊 Installation Summary:"
echo "Working tools: $tools_working/$total_tools"

if [[ $tools_working -eq $total_tools ]]; then
    log_success "All tools installed and working!"
else
    log_warning "Some tools are missing or not working"
    echo "Tests will still run but some checks may be skipped"
fi

echo ""
echo "🎯 Next steps:"
echo "1. Run tests: ./scripts/test-local.sh"
echo "2. Setup pre-commit: ./scripts/setup-pre-commit.sh"
echo "3. Read testing guide: cat TESTING.md"
