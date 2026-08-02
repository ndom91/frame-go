.PHONY: build-arm build-arm-docker build-native clean deploy test flake-build help

# Configuration
PI_HOST ?= pi@10.0.1.10
PI_TARGET_DIR ?= /home/pi
BINARY_NAME = photo-frame
FYNE_CROSS ?= $(shell go env GOPATH)/bin/fyne-cross

# Default target
help:
	@echo "📸 Photo Frame Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build-arm     - Cross-compile for ARM (Pi Zero 2W)"
	@echo "  build-arm-docker - Cross-compile for ARM using fyne-cross"
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
	@echo -e "\n🔨 Cross-compiling for ARM..."
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-unknown-linux-gnu-gcc go build -v -ldflags="-s -w -a -I=/lib/ld-linux-aarch64.so.1"  -o ${BINARY_NAME}-arm .
	@echo "✅ ARM binary ready: ${BINARY_NAME}-arm"

# Cross-compile for ARM using the fyne-cross Docker image.
build-arm-docker:
	${FYNE_CROSS} linux -arch=arm64 -output ${BINARY_NAME}-arm .
	tar -xJOf fyne-cross/dist/linux-arm64/${BINARY_NAME}-arm.tar.xz usr/local/bin/${BINARY_NAME} > ${BINARY_NAME}-arm

# Build for native system
build-native:
	@echo -e "\n🔨 Building for native system..."
	go build -o ${BINARY_NAME} .
	@echo "✅ Native binary ready: ${BINARY_NAME}"

# # Build native using Nix flake
# flake-build-native:
# 	@echo -e "\n❄️  Building native with Nix flake..."
# 	nix build .#photo-frame-native
# 	@echo "✅ Nix build complete"
# 	@ls -la result/bin/
#
# # Build ARM using Nix flake
# flake-build-arm:
# 	@echo -e "\n❄️  Building ARM with Nix flake..."
# 	nix build .#photo-frame-arm
# 	@echo "✅ Nix build complete"
# 	@ls -la result/bin/

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	rm -f ${BINARY_NAME} ${BINARY_NAME}-arm ${BINARY_NAME}-native
	rm -rf result fyne-cross
	go clean

# Deploy to Raspberry Pi
deploy: build-arm
	@echo "🚀 Deploying to Pi at ${PI_HOST}..."
	@scp -q -i ~/.ssh/id_ndo4 ${BINARY_NAME}-arm ${PI_HOST}:${PI_TARGET_DIR}/${BINARY_NAME}
	@echo "✅ Deployment complete"

# Development helpers
dev-native:
	nix develop

dev-arm:
	nix develop .#arm

# Quick development cycle
dev: build-native
	./${BINARY_NAME}

# Check if we're in the flake environment
check-env:
	@echo "Go version: $(shell go version)"
	@echo "GOOS: $(shell go env GOOS)"
	@echo "GOARCH: $(shell go env GOARCH)"
	@echo "CGO_ENABLED: $(shell go env CGO_ENABLED)"
