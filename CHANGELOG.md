# Changelog

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
