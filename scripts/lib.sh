#!/usr/bin/env bash

# Shared helpers for build scripts. Source this file, do not execute it.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

source "$ROOT_DIR/scripts/go-env.sh"

require_tool() {
	local tool="$1"
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "missing required tool: $tool" >&2
		exit 1
	fi
}

ldflags() {
	local flags="-s -w"
	if [[ -n "${VOLVID_VERSION:-}" ]]; then
		flags="$flags -X volvid/internal/adapters.Version=$VOLVID_VERSION"
	fi
	echo "$flags"
}
