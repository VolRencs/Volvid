#!/usr/bin/env python3
"""Generate latest.json and SHA256SUMS.txt for a Volvid release."""

from __future__ import annotations

import hashlib
import json
import os
import sys
from pathlib import Path

ASSETS = (
    ("linux-x86_64", "Volvid"),
    ("windows-x86_64", "Volvid.exe"),
)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> int:
    if len(sys.argv) != 4:
        print(
            f"usage: {sys.argv[0]} <dist-dir> <version> <tag>",
            file=sys.stderr,
        )
        return 2

    dist = Path(sys.argv[1])
    version = sys.argv[2]
    tag = sys.argv[3]
    repository = os.environ.get("GITHUB_REPOSITORY", "VolRencs/Volvid")
    base_url = f"https://github.com/{repository}/releases/download/{tag}"

    assets: dict[str, dict[str, object]] = {}
    checksums: list[tuple[str, Path]] = []
    for key, name in ASSETS:
        path = dist / name
        if not path.is_file():
            print(f"missing release asset: {path}", file=sys.stderr)
            return 1
        digest = sha256(path)
        assets[key] = {
            "url": f"{base_url}/{name}",
            "size": path.stat().st_size,
            "sha256": digest,
        }
        checksums.append((digest, path))

    manifest = {"version": version, "assets": assets}
    (dist / "latest.json").write_text(
        json.dumps(manifest, indent=2) + "\n", encoding="utf-8"
    )
    with (dist / "SHA256SUMS.txt").open("w", encoding="utf-8") as handle:
        for digest, path in checksums:
            handle.write(f"{digest}  {path.name}\n")

    print(json.dumps(manifest, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
