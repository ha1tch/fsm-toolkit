#!/bin/bash
# Cross-compile fsm-toolkit for multiple platforms
# Creates release archives in ./dist

set -e

VERSION="${VERSION:-$(git describe --tags 2>/dev/null || echo "dev")}"
DIST_DIR="./dist"
NAME="fsm-toolkit"

# Platforms to build
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

LDFLAGS="-s -w -X main.version=$VERSION"

echo "Building $NAME $VERSION for all platforms..."
echo ""

for PLATFORM in "${PLATFORMS[@]}"; do
    OS="${PLATFORM%/*}"
    ARCH="${PLATFORM#*/}"
    
    OUTPUT_DIR="$DIST_DIR/${NAME}-${VERSION}-${OS}-${ARCH}"
    mkdir -p "$OUTPUT_DIR"
    
    EXT=""
    if [ "$OS" = "windows" ]; then
        EXT=".exe"
    fi
    
    echo "Building for $OS/$ARCH..."
    
    GOOS="$OS" GOARCH="$ARCH" go build -ldflags "$LDFLAGS" \
        -o "$OUTPUT_DIR/fsm${EXT}" ./cmd/fsm/
    
    GOOS="$OS" GOARCH="$ARCH" go build -ldflags "$LDFLAGS" \
        -o "$OUTPUT_DIR/fsmedit${EXT}" ./cmd/fsmedit/
    
    # Copy docs
    cp README.md MANUAL.md "$OUTPUT_DIR/"
    cp -r examples "$OUTPUT_DIR/"
    
    # Create archive
    cd "$DIST_DIR"
    if [ "$OS" = "windows" ]; then
        zip -rq "${NAME}-${VERSION}-${OS}-${ARCH}.zip" "${NAME}-${VERSION}-${OS}-${ARCH}"
    else
        tar -czf "${NAME}-${VERSION}-${OS}-${ARCH}.tar.gz" "${NAME}-${VERSION}-${OS}-${ARCH}"
    fi
    rm -rf "${NAME}-${VERSION}-${OS}-${ARCH}"
    cd ..
done

echo ""
echo "Release archives:"
ls -lh "$DIST_DIR"
