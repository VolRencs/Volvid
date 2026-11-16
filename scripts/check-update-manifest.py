#!/usr/bin/env python3
"""Fetch a Volvid update manifest and verify its version."""

from __future__ import annotations

import json
import sys
import urllib.request


def main() -> int:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <manifest-url> <expected-version>", file=sys.stderr)
        return 2

    url, expected = sys.argv[1], sys.argv[2]
    request = urllib.request.Request(url, headers={"User-Agent": "volvid-release-check"})
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            manifest = json.load(response)
    except Exception as error:  # noqa: BLE001 - report any fetch/parse failure to CI
        print(f"could not read {url}: {error}", file=sys.stderr)
        return 1

    actual = manifest.get("version")
    if actual != expected:
        print(f"manifest version {actual!r} does not match {expected!r}", file=sys.stderr)
        return 1

    assets = manifest.get("assets")
    if not isinstance(assets, dict):
        print("manifest has no asset map", file=sys.stderr)
        return 1
    for platform in ("linux-x86_64", "windows-x86_64"):
        asset = assets.get(platform)
        if not isinstance(asset, dict):
            print(f"manifest is missing {platform}", file=sys.stderr)
            return 1
        for field in ("url", "size", "sha256"):
            if field not in asset:
                print(f"manifest asset {platform} is missing {field}", file=sys.stderr)
                return 1

    print(f"verified manifest {expected}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
