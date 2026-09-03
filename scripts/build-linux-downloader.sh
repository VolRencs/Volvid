#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_PATH="${1:-$ROOT_DIR/Volvid}"

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

LD_FLAGS="-s -w"
if [[ -n "${VOLVID_VERSION:-}" ]]; then
	LD_FLAGS="$LD_FLAGS -X volvid/internal/app.Version=$VOLVID_VERSION"
fi

GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" go build -trimpath -buildvcs=false -ldflags="$LD_FLAGS" -o "$OUTPUT_PATH" "$ROOT_DIR/cmd/downloader"
