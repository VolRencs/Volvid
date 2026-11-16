# Changelog

## 8.1.4

### Added

- Downloads use four parallel range connections per file through
  `ytrust --concurrent-fragments 4`, speeding up direct HTTP video and audio
  downloads.

### Changed

- Requires **ytrust 1.3.0** or newer in the 1.x series. ytrust 1.2 adds the
  `tv_samsung` client and 1.3 adds parallel range downloads.
- Automatic ytrust installation falls back to the required `v1.3.0` release
  tag if GitHub's "latest" pointer has not caught up yet.

### Fixed

- Cancelling a download now removes its leftover `.part` files and ytrust
  temporary files from the output folder.

## 8.1.3

### Added

- Update manifest (`latest.json`) published with every release, containing
  per-platform downloads, sizes and SHA-256 checksums.
- `--version` and `--update-check [--json]` diagnostics.
- Periodic update checks with retry backoff, visible check errors and a
  downloadable-update progress dialog with cancellation.
- Integrity checks and a health check before an update is installed.

### Changed

- Releases are built and published atomically from version tags; the GitHub
  API is only a fallback for older releases.
- Update checks prefer `releases/latest/download/latest.json`, avoiding the
  GitHub API rate limit and the gap while release assets are uploading.
- The updater uses a verified staging file and an atomic swap with rollback
  instead of `self-replace`.
- Restarting after an update now exits the old process deterministically.
- Updated all direct dependencies to their latest stable releases.

### Fixed

- Updates no longer appear missing when a release is published before its
  assets finish uploading.
- Corrupted or truncated update downloads are rejected instead of replacing
  the working executable.
- Read-only installation folders now produce an actionable error.
- Stale update files are cleaned up on startup.

## 8.1.2

### Changed

- Requires **ytrust 1.1.2** or newer in the 1.x series. ytrust now retries
  cold CDN `403`s before falling back to a muxed stream, so separate
  video/audio downloads stay at the requested quality.
- The fallback notification now says that YouTube's CDN rejected the stream
  instead of claiming that YouTube blocked separate streams.

## 8.1.1

### Fixed

- Linux release builds install `libxkbcommon-x11-dev`. GPUI links
  `libxkbcommon-x11` even in the Wayland-only build, so the previous CI runner
  failed at the linker step.

## 8.1.0

Volvid 8.1 is the first release built on the rewritten Rust/GPUI codebase from
the `Dev` branch.

### Added

- GPUI desktop application with dark/light themes and embedded SVG assets.
- Full ytrust integration: probing, quality scan, playlists, search, downloads,
  thumbnails, subtitles, dubbed audio tracks and fragments.
- Download queue with pause/resume/cancel, ETA, playlist workers and history.
- Fragment editor with URL timestamps and custom ranges.
- Dependency manager for managed ytrust, ffmpeg and Node.js.
- Browser cookie detection and JS runtime checks for YouTube challenges.
- English and Russian interface, switchable without restarting.
- In-app update check, restart flow and global update badge.
- Live engine checks: `search`, `formats`, `install` and `cancel` examples.

### Changed

- Requires **ytrust 1.1.1** or newer in the 1.x series.
- Linux builds are Wayland-only; the X11 backend is no longer compiled.
- Release assets are embedded, so the binary no longer depends on the build tree.
- Downloads rely on ytrust's native muxed fallback when YouTube rejects adaptive
  streams, and show the fallback state in the queue.

### Fixed

- Visible URL, playlist range, quality, track, thumbnail and dependency errors.
- Queue actions, playlist counts, history metadata, modal focus and text input
  behaviour.
- FFmpeg version parsing for nightly/static builds.
- Wayland/Vulkan build dependencies in CI.
