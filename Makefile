# Makefile for fail2ban-notify

BINARY_NAME := fail2ban-notify
VERSION := 1.0.0
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d " " -f 3)

# Build flags
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GoVersion=$(GO_VERSION)
BUILD_FLAGS := -ldflags "$(LDFLAGS)" -trimpath

# Directories
BUILD_DIR := build

.PHONY: all build clean install uninstall

# Default target
all: build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/fail2ban-notify

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
	rm -rf $(BUILD_DIR)
