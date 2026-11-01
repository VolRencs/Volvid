#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_PATH="${1:-$ROOT_DIR/Volvid}"

cd "$ROOT_DIR"
cargo build --release --locked -p volvid-gui
mkdir -p "$(dirname "$OUTPUT_PATH")"
cp "$ROOT_DIR/target/release/Volvid" "$OUTPUT_PATH"
chmod +x "$OUTPUT_PATH"
echo "Built $OUTPUT_PATH"
