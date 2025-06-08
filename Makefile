.PHONY: build-arm build-native clean deploy test flake-build help

# Configuration
PI_HOST ?= pi@raspberrypi.local
PI_TARGET_DIR ?= /home/pi/photo-frame
BINARY_NAME = photo-frame

# Default target
help:
	@echo "📸 Photo Frame Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build-arm     - Cross-compile for ARM (Pi Zero 2W)"
	@echo "  build-native  - Build for current system"
	@echo "  flake-build   - Build using Nix flake"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  deploy        - Deploy ARM binary to Pi"
	@echo "  ssh           - SSH into Pi"
	@echo ""
	@echo "Environment Variables:"
	@echo "  PI_HOST=${PI_HOST}"
	@echo "  PI_TARGET_DIR=${PI_TARGET_DIR}"

# Cross-compile for ARM using Go directly
build-arm:
	@echo "🔨 Cross-compiling for ARM..."
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 go build -ldflags="-s -w" -o ${BINARY_NAME}-arm .
	@echo "✅ ARM binary ready: ${BINARY_NAME}-arm"
	@file ${BINARY_NAME}-arm

# Build for native system
build-native:
	@echo "🔨 Building for native system..."
	go build -o ${BINARY_NAME} .
	@echo "✅ Native binary ready: ${BINARY_NAME}"

# Build using Nix flake (recommended)
flake-build:
	@echo "❄️  Building with Nix flake..."
	nix build .#photo-frame-arm
	@echo "✅ Nix build complete"
	@ls -la result/bin/

# Run tests
test:
	@echo "🧪 Running tests..."
	go test ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	rm -f ${BINARY_NAME} ${BINARY_NAME}-arm
	rm -rf result
	go clean

# Deploy to Raspberry Pi
deploy: build-arm
	@echo "🚀 Deploying to Pi at ${PI_HOST}..."
	ssh ${PI_HOST} "mkdir -p ${PI_TARGET_DIR}"
	scp ${BINARY_NAME}-arm ${PI_HOST}:${PI_TARGET_DIR}/${BINARY_NAME}
	scp config.example.json ${PI_HOST}:${PI_TARGET_DIR}/ || true
	@echo "✅ Deployment complete"
	@echo "Run on Pi: ssh ${PI_HOST} 'cd ${PI_TARGET_DIR} && ./${BINARY_NAME}'"

# Deploy using Nix-built binary
deploy-nix: flake-build
	@echo "🚀 Deploying Nix-built binary to Pi..."
	ssh ${PI_HOST} "mkdir -p ${PI_TARGET_DIR}"
	scp result/bin/photo-frame ${PI_HOST}:${PI_TARGET_DIR}/
	@echo "✅ Nix deployment complete"

# SSH into Pi
ssh:
	ssh ${PI_HOST}

# Development helpers
dev-shell:
	nix develop

dev-shell-native:
	nix develop .#native

# Quick development cycle
dev: build-native
	./${BINARY_NAME}

# Check if we're in the flake environment
check-env:
	@echo "Go version: $(shell go version)"
	@echo "GOOS: $(shell go env GOOS)"
	@echo "GOARCH: $(shell go env GOARCH)"
	@echo "CGO_ENABLED: $(shell go env CGO_ENABLED)"
