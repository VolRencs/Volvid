#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_PATH="${1:-$ROOT_DIR/VolRenDownloader_linux_amd64}"

source "$ROOT_DIR/scripts/go-env.sh"

require_tool() {
	local tool="$1"
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "missing required tool: $tool" >&2
		exit 1
	fi
}

require_tool go
mkdir -p "$(dirname "$OUTPUT_PATH")"

GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$OUTPUT_PATH" "$ROOT_DIR/cmd/downloader"
