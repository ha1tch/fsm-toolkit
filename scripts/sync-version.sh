#!/usr/bin/env bash
# sync-version.sh — Keep VERSION file and pkg/version/version.go in sync.
#
# Usage:
#   ./scripts/sync-version.sh          # Update version.go from VERSION
#   ./scripts/sync-version.sh --check  # Verify they match (for CI)
#   ./scripts/sync-version.sh --bump patch|minor|major  # Bump and sync

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION_FILE="$REPO_ROOT/VERSION"
VERSION_GO="$REPO_ROOT/pkg/version/version.go"

die() { echo "error: $*" >&2; exit 1; }

# Read current VERSION file
read_version() {
    [[ -f "$VERSION_FILE" ]] || die "VERSION file not found: $VERSION_FILE"
    tr -d '[:space:]' < "$VERSION_FILE"
}

# Read version from version.go
read_go_version() {
    [[ -f "$VERSION_GO" ]] || die "version.go not found: $VERSION_GO"
    grep 'const Version' "$VERSION_GO" | sed 's/.*"\(.*\)".*/\1/'
}

# Write version to version.go
write_go_version() {
    local ver="$1"
    cat > "$VERSION_GO" << EOF
// Package version provides the canonical version string for fsm-toolkit.
// The Version constant is kept in sync with the VERSION file at the
// repository root by running: ./scripts/sync-version.sh
package version

// Version is the current release version of fsm-toolkit.
const Version = "$ver"
EOF
}

# Write VERSION file
write_version_file() {
    printf '%s\n' "$1" > "$VERSION_FILE"
}

# Bump version
bump_version() {
    local ver="$1" part="$2"
    local major minor patch
    IFS='.' read -r major minor patch <<< "$ver"
    case "$part" in
        major) major=$((major + 1)); minor=0; patch=0 ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        patch) patch=$((patch + 1)) ;;
        *) die "unknown bump target: $part (use patch, minor, or major)" ;;
    esac
    echo "${major}.${minor}.${patch}"
}

case "${1:-sync}" in
    --check)
        file_ver="$(read_version)"
        go_ver="$(read_go_version)"
        if [[ "$file_ver" != "$go_ver" ]]; then
            die "version mismatch: VERSION=$file_ver, version.go=$go_ver — run ./scripts/sync-version.sh"
        fi
        echo "ok: version $file_ver"
        ;;
    --bump)
        [[ -n "${2:-}" ]] || die "usage: sync-version.sh --bump patch|minor|major"
        current="$(read_version)"
        new="$(bump_version "$current" "$2")"
        write_version_file "$new"
        write_go_version "$new"
        echo "$current -> $new"
        ;;
    sync|"")
        ver="$(read_version)"
        write_go_version "$ver"
        echo "synced: $ver"
        ;;
    *)
        die "usage: sync-version.sh [--check | --bump patch|minor|major]"
        ;;
esac
