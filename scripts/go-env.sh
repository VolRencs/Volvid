#!/usr/bin/env bash

case "$(uname -s 2>/dev/null || echo unknown)" in
	MINGW*|MSYS*|CYGWIN*)
		# Native Go on Windows expects Windows-style cache paths; keep its defaults.
		return 0 2>/dev/null || exit 0
		;;
esac

: "${VOLREN_GO_CACHE_ROOT:=/tmp}"
export GOCACHE="${GOCACHE:-$VOLREN_GO_CACHE_ROOT/volren-go-build}"
export GOMODCACHE="${GOMODCACHE:-$VOLREN_GO_CACHE_ROOT/volren-go-mod}"
mkdir -p "$GOCACHE" "$GOMODCACHE"
