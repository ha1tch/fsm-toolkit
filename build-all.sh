#!/bin/bash
# Cross-compile fsm-toolkit for all release platforms.
# Creates distribution archives in ./dist
#
# Usage:
#   ./build-all.sh              # build all platforms
#   ./build-all.sh linux/amd64  # build one platform

set -euo pipefail

VERSION="${VERSION:-$(cat VERSION 2>/dev/null || echo "dev")}"
if command -v git &>/dev/null && git rev-parse --git-dir &>/dev/null; then
    GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo "")"
    if [ -n "$GIT_SHA" ] && [[ "$VERSION" != *"$GIT_SHA"* ]]; then
        VERSION="${VERSION}-${GIT_SHA}"
    fi
fi

DIST_DIR="./dist"
NAME="fsm-toolkit"
LDFLAGS="-s -w -X github.com/ha1tch/fsm-toolkit/pkg/version.Version=${VERSION}"

# All supported platforms.
ALL_PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm/7"       # Raspberry Pi 2/3/4/5 (32-bit OS)
    "linux/arm/6"       # Raspberry Pi Zero / original Pi
    "darwin/amd64"      # macOS Intel
    "darwin/arm64"      # macOS Apple Silicon
    "windows/amd64"
    "windows/arm64"
    "freebsd/amd64"
    "freebsd/arm64"
    "openbsd/amd64"
    "netbsd/amd64"
)

# Use argument as filter, or build all.
if [ $# -gt 0 ]; then
    PLATFORMS=("$@")
else
    PLATFORMS=("${ALL_PLATFORMS[@]}")
fi

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "Building ${NAME} ${VERSION}"
echo "Platforms: ${#PLATFORMS[@]}"
echo ""

BUILT=0
FAILED=0

for PLATFORM in "${PLATFORMS[@]}"; do
    # Parse os/arch or os/arch/arm_version.
    IFS='/' read -r OS ARCH ARMV <<< "$PLATFORM"

    ARCH_LABEL="$ARCH"
    export GOARM=""
    if [ "$ARCH" = "arm" ] && [ -n "$ARMV" ]; then
        export GOARM="$ARMV"
        ARCH_LABEL="armv${ARMV}"
    fi

    DIST_NAME="${NAME}-${VERSION}-${OS}-${ARCH_LABEL}"
    echo -n "  ${OS}/${ARCH_LABEL}..."

    EXT=""
    if [ "$OS" = "windows" ]; then EXT=".exe"; fi

    BUILD_DIR="$DIST_DIR/${DIST_NAME}"
    mkdir -p "$BUILD_DIR"

    # Build both binaries.
    if ! GOOS="$OS" GOARCH="$ARCH" CGO_ENABLED=0 \
        go build -trimpath -ldflags "$LDFLAGS" \
        -o "$BUILD_DIR/fsm${EXT}" ./cmd/fsm/ 2>/dev/null; then
        echo " FAILED (fsm)"
        rm -rf "$BUILD_DIR"
        FAILED=$((FAILED + 1))
        continue
    fi

    if ! GOOS="$OS" GOARCH="$ARCH" CGO_ENABLED=0 \
        go build -trimpath -ldflags "$LDFLAGS" \
        -o "$BUILD_DIR/fsmedit${EXT}" ./cmd/fsmedit/ 2>/dev/null; then
        echo " FAILED (fsmedit)"
        rm -rf "$BUILD_DIR"
        FAILED=$((FAILED + 1))
        continue
    fi

    # Bundle documentation, examples, and class libraries.
    for f in LICENSE README.md MANUAL.md WORKFLOWS.md SPECIFICATION.md CHANGELOG.md \
             MACHINES.md COMPATIBILITY.md; do
        [ -f "$f" ] && cp "$f" "$BUILD_DIR/"
    done
    # Per-tool manuals (flattened for the distribution).
    [ -f cmd/fsm/MANUAL.md ] && cp cmd/fsm/MANUAL.md "$BUILD_DIR/FSM-CLI-MANUAL.md"
    [ -f cmd/fsmedit/MANUAL.md ] && cp cmd/fsmedit/MANUAL.md "$BUILD_DIR/FSMEDIT-MANUAL.md"
    cp -r examples "$BUILD_DIR/"
    cp -r class-libraries "$BUILD_DIR/"

    # Create archive.
    cd "$DIST_DIR"
    if [ "$OS" = "windows" ]; then
        zip -rq "${DIST_NAME}.zip" "${DIST_NAME}"
    else
        tar -czf "${DIST_NAME}.tar.gz" "${DIST_NAME}"
    fi
    rm -rf "${DIST_NAME}"
    cd ..

    BUILT=$((BUILT + 1))
    echo " ok"
done

echo ""
echo "Built: ${BUILT}  Failed: ${FAILED}"
echo ""

# Generate checksums.
cd "$DIST_DIR"
if command -v sha256sum &>/dev/null; then
    sha256sum *.tar.gz *.zip 2>/dev/null > SHA256SUMS.txt || true
    echo "Checksums written to dist/SHA256SUMS.txt"
elif command -v shasum &>/dev/null; then
    shasum -a 256 *.tar.gz *.zip 2>/dev/null > SHA256SUMS.txt || true
    echo "Checksums written to dist/SHA256SUMS.txt"
fi
cd ..

echo ""
echo "Archives:"
ls -lh "$DIST_DIR"/*.tar.gz "$DIST_DIR"/*.zip 2>/dev/null
