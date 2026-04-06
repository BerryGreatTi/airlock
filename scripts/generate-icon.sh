#!/usr/bin/env bash
# generate-icon.sh -- build AppIcon.icns from the programmatic Airlock icon.
#
# Compiles scripts/generate-icon-main.swift, runs it to emit a populated
# .iconset directory, then runs iconutil -c icns to produce AppIcon.icns.
#
# Usage: scripts/generate-icon.sh <output-icns-path>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SWIFT_SOURCE="$SCRIPT_DIR/generate-icon-main.swift"

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <output-icns-path>" >&2
    exit 1
fi

OUTPUT_ICNS="$1"
OUTPUT_DIR="$(dirname "$OUTPUT_ICNS")"
mkdir -p "$OUTPUT_DIR"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/airlock-icon.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

TOOL_BIN="$TMP_DIR/generate-icon"
ICONSET_DIR="$TMP_DIR/AppIcon.iconset"

echo "==> Compiling icon generator"
swiftc -o "$TOOL_BIN" \
    -framework AppKit \
    -framework SwiftUI \
    "$SWIFT_SOURCE"

echo "==> Rendering PNGs"
"$TOOL_BIN" "$ICONSET_DIR"

echo "==> Building .icns"
iconutil -c icns "$ICONSET_DIR" -o "$OUTPUT_ICNS"

echo "==> Wrote $OUTPUT_ICNS"
