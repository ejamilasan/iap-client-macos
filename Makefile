.PHONY: dev build build-universal clean install-deps generate-bindings

# Application name (no quotes -- quote at point of use for shell expansion)
APP_NAME := IAP Client
BUNDLE_ID := com.byteloomsolutions.iap-client

# Build directories
BUILD_DIR := build/bin
DMG_DIR := build/dmg

# Wails binary -- use GOBIN/GOPATH if not already in PATH
WAILS := $(shell command -v wails 2>/dev/null || echo "$(shell go env GOPATH)/bin/wails")

# Default target
all: build

# Install development dependencies
install-deps:
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "Installing frontend dependencies..."
	cd frontend && npm install
	@echo "Installing Wails CLI..."
	go install github.com/wailsapp/wails/v2/cmd/wails@latest
	@echo ""
	@echo "Done! If 'make dev' fails with 'wails: No such file', add Go's bin"
	@echo "directory to your PATH:"
	@echo ""
	@echo "  export PATH=\"\$$HOME/go/bin:\$$PATH\""
	@echo ""
	@echo "Add the line above to your ~/.zshrc to make it permanent."

# Generate Wails bindings
generate-bindings:
	$(WAILS) generate module

# Run in development mode
dev:
	$(WAILS) dev

# Build for production (current architecture)
build:
	$(WAILS) build

# Build universal binary (Intel + Apple Silicon)
build-universal:
	$(WAILS) build -platform darwin/universal

# Build and create DMG
build-dmg: build
	@echo "Creating DMG..."
	@mkdir -p "$(DMG_DIR)"
	@rm -f "$(DMG_DIR)/$(APP_NAME).dmg"
	@hdiutil create -volname "$(APP_NAME)" \
		-srcfolder "$(BUILD_DIR)/$(APP_NAME).app" \
		-ov -format UDZO \
		"$(DMG_DIR)/$(APP_NAME).dmg"
	@echo "DMG created at $(DMG_DIR)/$(APP_NAME).dmg"

# Build universal DMG
build-dmg-universal: build-universal
	@echo "Creating universal DMG..."
	@mkdir -p "$(DMG_DIR)"
	@rm -f "$(DMG_DIR)/$(APP_NAME)-universal.dmg"
	@hdiutil create -volname "$(APP_NAME)" \
		-srcfolder "$(BUILD_DIR)/$(APP_NAME).app" \
		-ov -format UDZO \
		"$(DMG_DIR)/$(APP_NAME)-universal.dmg"
	@echo "Universal DMG created at $(DMG_DIR)/$(APP_NAME)-universal.dmg"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DMG_DIR)
	@rm -rf frontend/dist
	@rm -rf frontend/node_modules
	@echo "Clean complete"

# Run tests
test:
	go test -v ./...

# Run linting
lint:
	golangci-lint run
	cd frontend && npm run lint

# Format code
fmt:
	go fmt ./...
	cd frontend && npm run format

# Check for security issues
security-check:
	gosec ./...

# Update dependencies
update-deps:
	go get -u ./...
	go mod tidy
	cd frontend && npm update

# Show help
help:
	@echo "IAP Client for macOS - Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make install-deps      Install all dependencies"
	@echo "  make dev               Run in development mode"
	@echo "  make build             Build for production"
	@echo "  make build-universal   Build universal binary (Intel + Apple Silicon)"
	@echo "  make build-dmg         Build and create DMG installer"
	@echo "  make clean             Clean build artifacts"
	@echo "  make test              Run tests"
	@echo "  make lint              Run linters"
	@echo "  make fmt               Format code"
	@echo "  make help              Show this help"
