#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

OUTPUT_PATH="${1:-$ROOT_DIR/Volvid}"

require_tool go
mkdir -p "$(dirname "$OUTPUT_PATH")"

GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" go build -trimpath -buildvcs=false -ldflags="$(ldflags)" -o "$OUTPUT_PATH" "$ROOT_DIR/cmd/downloader"
