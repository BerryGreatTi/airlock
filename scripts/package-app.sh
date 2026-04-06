#!/usr/bin/env bash
# package-app.sh -- build Airlock.app bundle with embedded Go CLI.
#
# Produces a .app bundle at <output-dir>/Airlock.app containing:
#   Contents/MacOS/AirlockApp   (Swift GUI binary)
#   Contents/MacOS/airlock      (Go CLI binary)
#   Contents/Resources/AppIcon.icns
#   Contents/Info.plist
#
# The bundle is ad-hoc code signed by default. Pass --sign <identity> to use
# a Developer ID certificate. When a non-ad-hoc identity is provided, the
# hardened runtime and timestamp are enabled for notarization readiness.
set -euo pipefail

usage() {
    cat <<'EOF'
Usage: scripts/package-app.sh [options]

Options:
  --version <ver>        Version string to inject into Info.plist (default: dev)
  --cli-binary <path>    Pre-built Go CLI binary (skip go build if provided)
  --output <dir>         Output directory (default: build)
  --sign <identity>      codesign identity (default: - for ad-hoc)
  -h, --help             Show this help
EOF
}

# --- Paths ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

APP_NAME="Airlock"
SWIFT_TARGET="AirlockApp"
PLIST_TEMPLATE="$REPO_ROOT/AirlockApp/Resources/Info.plist"
GO_PACKAGE="./cmd/airlock"
GO_VERSION_VAR="github.com/taeikkim92/airlock/internal/cli.Version"

# --- Defaults ---
VERSION="dev"
CLI_BINARY=""
OUTPUT_DIR="$REPO_ROOT/build"
SIGN_IDENTITY="-"

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --cli-binary)
            CLI_BINARY="$2"
            shift 2
            ;;
        --output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --sign)
            SIGN_IDENTITY="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

APP_BUNDLE="$OUTPUT_DIR/$APP_NAME.app"
CONTENTS="$APP_BUNDLE/Contents"
MACOS_DIR="$CONTENTS/MacOS"
RESOURCES_DIR="$CONTENTS/Resources"

echo "==> Output: $APP_BUNDLE"
echo "==> Version: $VERSION"

# --- Prepare output directory ---
mkdir -p "$OUTPUT_DIR"
rm -rf "$APP_BUNDLE"
mkdir -p "$MACOS_DIR" "$RESOURCES_DIR"

# --- Build or locate Go CLI ---
if [[ -z "$CLI_BINARY" ]]; then
    echo "==> Building Go CLI"
    CLI_BINARY="$OUTPUT_DIR/airlock-cli"
    (
        cd "$REPO_ROOT"
        go build \
            -ldflags "-X $GO_VERSION_VAR=$VERSION" \
            -o "$CLI_BINARY" \
            "$GO_PACKAGE"
    )
else
    if [[ ! -f "$CLI_BINARY" ]]; then
        echo "Error: --cli-binary path does not exist: $CLI_BINARY" >&2
        exit 1
    fi
fi

# --- Build Swift GUI ---
# `swift build --show-bin-path` triggers a build if artifacts are stale, so a
# single invocation both builds and reveals the output directory.
echo "==> Building Swift GUI (release)"
SWIFT_BIN_DIR="$(cd "$REPO_ROOT/AirlockApp" && swift build -c release --show-bin-path)"
SWIFT_BINARY="$SWIFT_BIN_DIR/$SWIFT_TARGET"

if [[ ! -f "$SWIFT_BINARY" ]]; then
    echo "Error: Swift binary not found at $SWIFT_BINARY" >&2
    exit 1
fi

# --- Generate icon ---
echo "==> Generating AppIcon.icns"
"$SCRIPT_DIR/generate-icon.sh" "$RESOURCES_DIR/AppIcon.icns"

# --- Copy binaries ---
echo "==> Copying binaries into bundle"
cp "$SWIFT_BINARY" "$MACOS_DIR/$SWIFT_TARGET"
cp "$CLI_BINARY" "$MACOS_DIR/airlock"
chmod +x "$MACOS_DIR/$SWIFT_TARGET" "$MACOS_DIR/airlock"

# --- Install Info.plist with version ---
echo "==> Writing Info.plist"
cp "$PLIST_TEMPLATE" "$CONTENTS/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$CONTENTS/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$CONTENTS/Info.plist"

# --- Code sign (inside-out, per Apple guidance) ---
# `codesign --deep` is deprecated and rejected by notarytool. Sign the helper
# binary first, then the bundle. For a Developer ID identity, enable the
# hardened runtime and secure timestamp (both required for notarization).
echo "==> Code signing (identity: $SIGN_IDENTITY)"
CODESIGN_FLAGS=(--force --sign "$SIGN_IDENTITY")
if [[ "$SIGN_IDENTITY" != "-" ]]; then
    CODESIGN_FLAGS+=(--options=runtime --timestamp)
fi

codesign "${CODESIGN_FLAGS[@]}" "$MACOS_DIR/airlock"
codesign "${CODESIGN_FLAGS[@]}" "$APP_BUNDLE"
codesign --verify --verbose "$APP_BUNDLE"

echo "==> Done: $APP_BUNDLE"
