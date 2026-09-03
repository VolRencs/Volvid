#!/usr/bin/env bash

case "$(uname -s 2>/dev/null || echo unknown)" in
	MINGW*|MSYS*|CYGWIN*)
		# Native Go on Windows expects Windows-style cache paths; keep its defaults.
		return 0 2>/dev/null || exit 0
		;;
esac

: "${VOLVID_GO_CACHE_ROOT:=/tmp}"
export GOCACHE="${GOCACHE:-$VOLVID_GO_CACHE_ROOT/volvid-go-build}"
export GOMODCACHE="${GOMODCACHE:-$VOLVID_GO_CACHE_ROOT/volvid-go-mod}"
mkdir -p "$GOCACHE" "$GOMODCACHE"
