#!/bin/bash
# Enhanced local testing script
# Usage: ./scripts/test-local.sh [options]
# Options:
#   --quick     Skip slow tests (linting, security scan)
#   --no-race   Skip race detection tests
#   --coverage  Generate coverage report
#   --fix       Auto-fix issues where possible
#   --verbose   Verbose output

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Options
QUICK_MODE=false
NO_RACE=false
COVERAGE_MODE=false
FIX_MODE=false
VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --quick)
            QUICK_MODE=true
            shift
            ;;
        --no-race)
            NO_RACE=true
            shift
            ;;
        --coverage)
            COVERAGE_MODE=true
            shift
            ;;
        --fix)
            FIX_MODE=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --quick     Skip slow tests (linting, security scan)"
            echo "  --no-race   Skip race detection tests"
            echo "  --coverage  Generate coverage report"
            echo "  --fix       Auto-fix issues where possible"
            echo "  --verbose   Verbose output"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Logging functions
log_step() {
    echo -e "${BLUE}🔄 $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_info() {
    echo -e "${PURPLE}ℹ️  $1${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Run a step and handle errors
run_step() {
    local step_name="$1"
    local step_command="$2"
    local allow_failure="${3:-false}"

    log_step "$step_name"

    if [[ "$VERBOSE" == "true" ]]; then
        echo "Running: $step_command"
    fi

    if eval "$step_command"; then
        log_success "$step_name completed"
        return 0
    else
        if [[ "$allow_failure" == "true" ]]; then
            log_warning "$step_name failed (non-critical)"
            return 0
        else
            log_error "$step_name failed"
            return 1
        fi
    fi
}

# Main testing function
main() {
    echo -e "${BLUE}🚀 Starting local tests for fail2ban-notify${NC}"
    echo "================================================="

    local failed_steps=0
    local start_time=$(date +%s)

    # Check Go installation
    if ! command_exists go; then
        log_error "Go is not installed"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}')
    log_info "Using Go version: $GO_VERSION"

    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod not found. Are you in the project root?"
        exit 1
    fi

    # Step 1: Clean previous artifacts
    log_step "Cleaning previous artifacts"
    rm -f coverage.out coverage.html
    rm -rf dist/
    log_success "Cleanup completed"

    # Step 2: Format check/fix
    if [[ "$FIX_MODE" == "true" ]]; then
        if ! run_step "Auto-formatting code" "find . -type f -name '*.go' ! -path './.history/*' ! -path './vendor/*' -print0 | xargs -0 gofmt -s -w"; then
            ((failed_steps++))
        fi
    else
        if ! run_step "Checking code format" "test -z \"\$(find . -type f -name '*.go' ! -path './.history/*' ! -path './vendor/*' -print0 | xargs -0 gofmt -s -l)\""; then
            log_warning "Code is not formatted. Run with --fix to auto-format, or run: go fmt ./..."
            ((failed_steps++))
        fi
    fi

    # Step 3: Vet check
    if ! run_step "Running go vet" "go vet ./..."; then
        ((failed_steps++))
    fi

    # Step 4: Dependency verification
    if ! run_step "Verifying dependencies" "go mod verify"; then
        ((failed_steps++))
    fi

    if [[ "$FIX_MODE" == "true" ]]; then
        if ! run_step "Tidying dependencies" "go mod tidy"; then
            ((failed_steps++))
        fi
    fi

    # Step 5: Linting (skip in quick mode)
    if [[ "$QUICK_MODE" != "true" ]]; then
        if command_exists golangci-lint; then
            if ! run_step "Running linter with Go 1.23.10" "go1.23.10 run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.58.1 run"; then
                ((failed_steps++))
            fi
        else
            log_warning "golangci-lint not installed. Install with:"
            log_info "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        fi
    fi

    # Step 6: Security scan (skip in quick mode)
    if [[ "$QUICK_MODE" != "true" ]]; then
        if command_exists gosec; then
            if ! run_step "Running security scan" "gosec ./..."; then
                log_warning "Security issues found. Review the output above."
                ((failed_steps++))
            fi
        else
            log_warning "gosec not installed. Install with:"
            log_info "go install github.com/securego/gosec/v2/cmd/gosec@latest"
        fi
    fi

    # Step 7: Unit tests
    local test_command=""
    local test_name=""

    # Check CGO availability
    local cgo_available=false
    if command_exists gcc && [[ "${CGO_ENABLED:-1}" != "0" ]] && [[ "$NO_RACE" != "true" ]]; then
        # Test if CGO works
        if echo 'package main; import "C"; func main() {}' | CGO_ENABLED=1 go run -x - 2>/dev/null; then
            cgo_available=true
        fi
    fi

    if [[ "$cgo_available" == "true" ]]; then
        test_command="CGO_ENABLED=1 go test -race -coverprofile=coverage.out ./..."
        test_name="Running tests with race detection"
    else
        test_command="CGO_ENABLED=0 go test -coverprofile=coverage.out ./..."
        test_name="Running tests (no race detection)"
        if [[ "$NO_RACE" != "true" ]]; then
            log_warning "CGO not available, running tests without race detection"
        fi
    fi

    if ! run_step "$test_name" "$test_command"; then
        ((failed_steps++))
    fi

    # Step 8: Coverage report
    if [[ -f coverage.out ]]; then
        if [[ "$COVERAGE_MODE" == "true" ]]; then
            if ! run_step "Generating coverage report" "go tool cover -html=coverage.out -o coverage.html"; then
                ((failed_steps++))
            fi

            if [[ -f coverage.html ]]; then
                log_info "Coverage report generated: coverage.html"
            fi
        fi

        # Show coverage summary
        local coverage_info=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
        if [[ -n "$coverage_info" ]]; then
            log_info "Test coverage: $coverage_info"
        fi
    fi

    # Step 9: Build verification
    if ! run_step "Building application" "go build -v ./cmd/fail2ban-notify"; then
        ((failed_steps++))
    fi

    # Step 10: Binary test
    if [[ -f fail2ban-notify ]]; then
        if ! run_step "Testing binary" "./fail2ban-notify -version"; then
            ((failed_steps++))
        fi

        # Test help (ignore exit code since help often returns non-zero)
        log_step "Testing help command"
        ./fail2ban-notify -h > /dev/null 2>&1 || true
        log_success "Help command tested"

        # Cleanup binary
        rm -f fail2ban-notify
    fi

    # Step 11: Cross-platform build test (skip in quick mode)
    if [[ "$QUICK_MODE" != "true" ]]; then
        if ! run_step "Cross-platform build test" "make build-all"; then
            # Try manual cross-compile
            log_info "Trying manual cross-compilation..."
            mkdir -p dist

            local platforms=(
                "linux/amd64"
                "linux/arm64"
                "darwin/amd64"
                "windows/amd64"
            )

            local cross_compile_success=true
            for platform in "${platforms[@]}"; do
                local os=$(echo "$platform" | cut -d'/' -f1)
                local arch=$(echo "$platform" | cut -d'/' -f2)
                local output="dist/fail2ban-notify-$os-$arch"

                if [[ "$os" == "windows" ]]; then
                    output="$output.exe"
                fi

                log_step "Building for $os/$arch"
                if CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o "$output" ./cmd/fail2ban-notify; then
                    log_success "Built for $os/$arch"
                else
                    log_error "Failed to build for $os/$arch"
                    cross_compile_success=false
                fi
            done

            if [[ "$cross_compile_success" != "true" ]]; then
                ((failed_steps++))
            fi
        fi
    fi

    # Summary
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo ""
    echo "================================================="
    log_info "Local testing completed in ${duration}s"

    if [[ $failed_steps -eq 0 ]]; then
        log_success "All tests passed! 🎉"
        echo ""
        log_info "Your code is ready for:"
        echo "  • Git commit and push"
        echo "  • Pull request creation"
        echo "  • CI pipeline"
        echo ""

        if [[ -f coverage.html ]]; then
            log_info "Open coverage report: open coverage.html"
        fi

        if [[ -d dist ]]; then
            log_info "Built binaries available in dist/"
            ls -la dist/ 2>/dev/null || true
        fi

        exit 0
    else
        log_error "$failed_steps step(s) failed"
        echo ""
        log_info "Fix the failing steps before pushing to CI"

        if [[ "$QUICK_MODE" == "true" ]]; then
            log_info "Run without --quick for complete testing"
        fi

        if [[ "$FIX_MODE" != "true" ]]; then
            log_info "Run with --fix to auto-fix some issues"
        fi

        exit 1
    fi
}

# Handle interruption
trap 'log_warning "Interrupted by user"; exit 130' INT

# Run main function
main "$@"
