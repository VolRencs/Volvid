# Releasing Volvid

Releases are built from tags. Do not create the GitHub release manually:
the tag starts the release workflow, which builds both platforms, generates
the update manifest, uploads every asset to a draft release, verifies them,
and only then publishes the release.

## Checklist

1. Update `[workspace.package].version` in `Cargo.toml`.
2. Add the release notes to `CHANGELOG.md`.
3. Run the local checks:

   ```bash
   cargo fmt --all -- --check
   cargo clippy --workspace --all-targets --locked -- -D warnings
   cargo test --workspace --locked
   cargo build --release --locked -p volvid-gui
   ./target/release/Volvid --update-check
   ```

4. Commit and push the version bump.
5. Create and push the tag `8.1.4` (no `v` prefix):

   ```bash
   git tag 8.1.4
   git push origin 8.1.4
   ```

6. Wait for the `Release` workflow. It publishes:

   | Asset | Purpose |
   |-------|---------|
   | `Volvid` | Linux x86-64 executable |
   | `Volvid.exe` | Windows x86-64 executable |
   | `latest.json` | Update manifest with sizes and SHA-256 checksums |
   | `SHA256SUMS.txt` | Plain-text checksums |

The workflow fails if the tag does not match the workspace version, if an
asset is missing, or if the public `latest.json` cannot be read after
publishing.

## Update manifest

`latest.json` is served from the stable URL:

```text
https://github.com/VolRencs/Volvid/releases/latest/download/latest.json
```

Clients read it without the rate-limited GitHub API. The API is only a
fallback for releases that predate the manifest. Format:

```json
{
  "version": "8.1.4",
  "assets": {
    "linux-x86_64": {
      "url": "https://github.com/VolRencs/Volvid/releases/download/8.1.4/Volvid",
      "size": 27390048,
      "sha256": "..."
    },
    "windows-x86_64": {
      "url": "https://github.com/VolRencs/Volvid/releases/download/8.1.4/Volvid.exe",
      "size": 18976768,
      "sha256": "..."
    }
  }
}
```

The client checks the size and SHA-256, verifies the executable format, runs
the downloaded file with `--version`, and only then performs an atomic
replacement. The previous executable is kept as a rollback copy until a
later start.

Generate the files locally with:

```bash
python3 scripts/release-manifest.py dist 8.1.4 8.1.4
```

## Diagnostics

```bash
Volvid --version
Volvid --update-check
Volvid --update-check --json
```

`VOLVID_UPDATE_MANIFEST` points the checker at a different manifest URL for
testing. The asset URLs inside it must still use the GitHub release host.

## Code signing

Windows builds are currently unsigned. SignPath Foundation can be added to
the publish workflow later if the project is accepted; sign the `.exe`
before generating `latest.json`, otherwise the published checksum will not
match. A signing step is intentionally not included yet.
