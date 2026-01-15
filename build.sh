#!/bin/bash
# Build script for fsm-toolkit
# Builds fsm and fsmedit commands

set -e

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
esac

# Output directory
OUT_DIR="${OUT_DIR:-./bin}"
mkdir -p "$OUT_DIR"

# Version (from git tag or "dev")
VERSION="${VERSION:-$(git describe --tags 2>/dev/null || echo "dev")}"

# Build flags
LDFLAGS="-s -w -X main.version=$VERSION"

echo "Building fsm-toolkit ($VERSION) for $OS/$ARCH..."

# Build fsm
echo "  Building fsm..."
go build -ldflags "$LDFLAGS" -o "$OUT_DIR/fsm" ./cmd/fsm/

# Build fsmedit
echo "  Building fsmedit..."
go build -ldflags "$LDFLAGS" -o "$OUT_DIR/fsmedit" ./cmd/fsmedit/

echo ""
echo "Built:"
ls -lh "$OUT_DIR"/fsm "$OUT_DIR"/fsmedit
echo ""
echo "Run: $OUT_DIR/fsm --help"
echo "     $OUT_DIR/fsmedit --help"
