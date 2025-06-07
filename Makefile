# Makefile for fail2ban-notify

BINARY_NAME := fail2ban-notify
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d " " -f 3)

# Build flags
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GoVersion=$(GO_VERSION)
BUILD_FLAGS := -ldflags "$(LDFLAGS)" -trimpath

# Directories
BUILD_DIR := build
DIST_DIR := dist

# Platforms to build for
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	linux/armv7 \
	darwin/amd64 \
	darwin/arm64

.PHONY: all build clean test lint install uninstall release help

# Default target
all: test build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building for $$OS/$$ARCH..."; \
		GOOS=$$OS GOARCH=$$ARCH go build $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-$$OS-$$ARCH .; \
		if [ $$? -ne 0 ]; then \
			echo "Failed to build for $$OS/$$ARCH"; \
			exit 1; \
		fi; \
	done

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Install locally
install: build
	@echo "Installing $(BINARY_NAME)..."
	sudo install -m 755 $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo install -m 644 configs/notify.conf /etc/fail2ban/action.d/ 2>/dev/null || echo "Fail2ban action config not installed (fail2ban may not be installed)"
	@echo "Initializing configuration..."
	sudo /usr/local/bin/$(BINARY_NAME) -init || echo "Could not initialize config (may need manual setup)"
	@echo "Installation complete!"

# Uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	sudo rm -f /etc/fail2ban/action.d/notify.conf
	@echo "Note: Configuration file /etc/fail2ban/fail2ban-notify.json left in place"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) $(DIST_DIR)

# Create release packages
release: build-all
	@echo "Creating release packages..."
	@mkdir -p $(DIST_DIR)/packages
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		PACKAGE_NAME=$(BINARY_NAME)-$(VERSION)-$$OS-$$ARCH; \
		echo "Creating package for $$OS/$$ARCH..."; \
		mkdir -p $(DIST_DIR)/$$PACKAGE_NAME; \
		cp $(DIST_DIR)/$(BINARY_NAME)-$$OS-$$ARCH $(DIST_DIR)/$$PACKAGE_NAME/$(BINARY_NAME); \
		cp configs/notify.conf $(DIST_DIR)/$$PACKAGE_NAME/; \
		cp README.md $(DIST_DIR)/$$PACKAGE_NAME/ 2>/dev/null || echo "README.md not found"; \
		cp LICENSE $(DIST_DIR)/$$PACKAGE_NAME/ 2>/dev/null || echo "LICENSE not found"; \
		cd $(DIST_DIR) && tar -czf packages/$$PACKAGE_NAME.tar.gz $$PACKAGE_NAME/; \
		cd ..; \
		rm -rf $(DIST_DIR)/$$PACKAGE_NAME; \
	done
	@echo "Release packages created in $(DIST_DIR)/packages/"

# Development targets
dev-setup:
	@echo "Setting up development environment..."
	go mod tidy
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi

run: build
	@echo "Running $(BINARY_NAME) with test parameters..."
	$(BUILD_DIR)/$(BINARY_NAME) -ip="192.168.1.100" -jail="test" -action="ban" -debug

# Quick installation script
quick-install:
	@echo "Creating quick installation script..."
	@cat > quick-install.sh << 'EOF'
#!/bin/bash
curl -fsSL https://raw.githubusercontent.com/eyeskiller/fail2ban-notifier/main/install.sh | bash
EOF
	@chmod +x quick-install.sh
	@echo "Quick install script created: ./quick-install.sh"

# Docker targets (optional)
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-run: docker-build
	docker run --rm $(BINARY_NAME):$(VERSION) -ip="192.168.1.100" -jail="test" -action="ban"

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build for current platform"
	@echo "  build-all     - Build for all supported platforms"
	@echo "  test          - Run tests"
	@echo "  lint          - Run linter"
	@echo "  install       - Install locally (requires sudo)"
	@echo "  uninstall     - Uninstall (requires sudo)"
	@echo "  clean         - Clean build artifacts"
	@echo "  release       - Create release packages"
	@echo "  dev-setup     - Setup development environment"
	@echo "  run           - Build and run with test parameters"
	@echo "  quick-install - Create quick installation script"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Build and run Docker container"
	@echo "  help          - Show this help"
