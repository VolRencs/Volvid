#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG_DIR="$ROOT_DIR/cmd/downloader"
ICON_FILE="$ROOT_DIR/assets/icon/icon.ico"
SYSO_FILE="$PKG_DIR/zz_build_windows_icon.syso"

OUTPUT_PATH="${1:-$ROOT_DIR/VolRenDownloader.exe}"

source "$ROOT_DIR/scripts/go-env.sh"

require_tool() {
	local tool="$1"
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "missing required tool: $tool" >&2
		exit 1
	fi
}

resolve_tool_candidate() {
	local tool="$1"
	local -a candidates=()
	local path_tool
	local go_bin

	if path_tool="$(command -v "$tool" 2>/dev/null)"; then
		echo "$path_tool"
		return 0
	fi

	if [[ -n "${GOBIN:-}" ]]; then
		candidates+=("$GOBIN/$tool" "$GOBIN/$tool.exe")
	fi

	go_bin="$(go env GOPATH 2>/dev/null)/bin"
	candidates+=(
		"$go_bin/$tool"
		"$go_bin/$tool.exe"
		"$HOME/go/bin/$tool"
		"$HOME/go/bin/$tool.exe"
	)

	for path_tool in "${candidates[@]}"; do
		if [[ -x "$path_tool" ]]; then
			echo "$path_tool"
			return 0
		fi
	done

	return 1
}

resolve_first_tool() {
	local path_tool
	local tool

	for tool in "$@"; do
		if path_tool="$(resolve_tool_candidate "$tool")"; then
			echo "$path_tool"
			return 0
		fi
	done

	echo "missing required tool: $1" >&2
	exit 1
}

cleanup() {
	rm -f "$SYSO_FILE"
}
trap cleanup EXIT

require_tool go
RSRC_TOOL="$(resolve_first_tool rsrc)"

if [[ ! -f "$ICON_FILE" ]]; then
	echo "missing icon file: $ICON_FILE" >&2
	exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

"$RSRC_TOOL" -ico "$ICON_FILE" -arch amd64 -o "$SYSO_FILE"

GOOS=windows GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$OUTPUT_PATH" ./cmd/downloader
