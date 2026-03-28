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

resolve_tool_candidate() {
	local tool="$1"
	local -a candidates=()
	local path_tool

	if path_tool="$(command -v "$tool" 2>/dev/null)"; then
		echo "$path_tool"
		return 0
	fi

	if [[ -n "${LLVM_BIN:-}" ]]; then
		candidates+=("$LLVM_BIN/$tool" "$LLVM_BIN/$tool.exe")
	fi

	candidates+=(
		"/c/Program Files/LLVM/bin/$tool"
		"/c/Program Files/LLVM/bin/$tool.exe"
		"/mnt/c/Program Files/LLVM/bin/$tool"
		"/mnt/c/Program Files/LLVM/bin/$tool.exe"
		"/c/ProgramData/chocolatey/lib/llvm/tools/llvm/bin/$tool"
		"/c/ProgramData/chocolatey/lib/llvm/tools/llvm/bin/$tool.exe"
		"/mnt/c/ProgramData/chocolatey/lib/llvm/tools/llvm/bin/$tool"
		"/mnt/c/ProgramData/chocolatey/lib/llvm/tools/llvm/bin/$tool.exe"
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

is_windows_binary() {
	local tool="$1"
	case "$(basename "$tool")" in
	*.exe | rc | cvtres)
		return 0
		;;
	*)
		return 1
		;;
	esac
}

native_path() {
	local path="$1"
	if command -v cygpath >/dev/null 2>&1; then
		cygpath -w "$path"
		return 0
	fi
	echo "$path"
}

tool_path_arg() {
	local tool="$1"
	local path="$2"
	if is_windows_binary "$tool"; then
		native_path "$path"
		return 0
	fi
	echo "$path"
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
RC_TOOL="$(resolve_first_tool rc llvm-rc)"
CVTRES_TOOL="$(resolve_first_tool cvtres llvm-cvtres)"

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
	"$RC_TOOL" /fo "$(tool_path_arg "$RC_TOOL" "$RES_FILE")" "$(basename "$RC_FILE")"
)

"$CVTRES_TOOL" "/MACHINE:$(machine_for_arch "$ARCH")" "/OUT:$(tool_path_arg "$CVTRES_TOOL" "$SYSO_FILE")" "$(tool_path_arg "$CVTRES_TOOL" "$RES_FILE")"

GOOS=windows GOARCH="$ARCH" go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$OUTPUT_PATH" ./cmd/downloader
