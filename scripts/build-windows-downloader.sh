#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG_DIR="$ROOT_DIR/cmd/downloader"
RC_FILE="$PKG_DIR/windows_icon.rc"
ICON_FILE="$ROOT_DIR/assets/icon/icon.ico"
TMP_DIR="$(mktemp -d)"
RES_FILE="$TMP_DIR/downloader_icon.res"
SYSO_FILE="$PKG_DIR/zz_build_windows_icon.syso"

ARCH="${GOARCH:-amd64}"
OUTPUT_PATH="${1:-$ROOT_DIR/VolRenDownloader.exe}"

require_tool() {
	local tool="$1"
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "missing required tool: $tool" >&2
		exit 1
	fi
}

machine_for_arch() {
	case "$1" in
	amd64)
		echo "X64"
		;;
	arm64)
		echo "ARM64"
		;;
	*)
		echo "unsupported GOARCH: $1" >&2
		exit 1
		;;
	esac
}

cleanup() {
	rm -f "$SYSO_FILE"
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT

require_tool go
require_tool llvm-rc
require_tool llvm-cvtres

if [[ ! -f "$RC_FILE" ]]; then
	echo "missing resource file: $RC_FILE" >&2
	exit 1
fi
if [[ ! -f "$ICON_FILE" ]]; then
	echo "missing icon file: $ICON_FILE" >&2
	exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

(
	cd "$PKG_DIR"
	llvm-rc /fo "$RES_FILE" "$(basename "$RC_FILE")"
)

llvm-cvtres "/MACHINE:$(machine_for_arch "$ARCH")" "/OUT:$SYSO_FILE" "$RES_FILE"

GOOS=windows GOARCH="$ARCH" go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$OUTPUT_PATH" ./cmd/downloader
