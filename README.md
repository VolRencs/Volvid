# Volvid

<div align="center">

<img src="docs/assets/logo.svg" alt="VOLVID" height="87">

**A fast, modern YouTube downloader for the desktop — built with Rust and GPUI.**

[![Rust](https://img.shields.io/badge/Rust-1.99.1%2B-000000?style=flat-square&logo=rust)](rust-toolchain.toml)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20Wayland-5CC8E8?style=flat-square)](#platforms)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue?style=flat-square)](LICENSE)

[![GUI](docs/screenshots/gui.png)](docs/screenshots/gui.png)

</div>

---

## About

Volvid is a graphical YouTube downloader powered by **ytrust** and **ffmpeg**.
It checks dependencies, resolves video metadata and thumbnails, scans real
quality options, and downloads video, audio or thumbnails — including playlists,
fragments, dubbed audio tracks and embedded subtitles.

The v8 release is a complete rewrite in Rust. The previous Go/TUI version has
been removed; the new app is a native GPU-rendered desktop application.

## Features

- **Modern GUI** — GPUI, dark/light themes, custom coral/cyan palette.
- **Video preview** — YouTube thumbnails are fetched and shown on the download screen.
- **Quality scan** — real heights, availability and estimated sizes from `ytrust`.
- **Formats** — Original MP4, H.264, H.265, VP9, AV1, MKV copy; MP3 320/192, M4A, Opus, FLAC.
- **Fallbacks** — ytrust retries blocked adaptive streams with a muxed format and Volvid shows the fallback in the queue.
- **Playlists** — virtualized list, filter, select all/none, manual ranges and parallel workers.
- **Tracks** — dubbed audio languages and subtitles embedded into MP4/MKV containers (WebM keeps sidecar files).
- **Fragments** — `1:00-2:30`, `1:00-`, `1:00+`, and timestamps from the URL.
- **Dependency manager** — detects and installs managed ytrust, ffmpeg and node.
- **Browser cookies** — auto-detects Firefox, Chrome, Chromium, Brave, Vivaldi and Opera profiles.
- **Self-update** — GitHub release check and in-place binary replacement.
- **Download queue** — queued/active/paused jobs, progress, speed, ETA, pause/resume and session history.
- **English / Russian** — switch language instantly from the UI.
- **Wayland-native Linux** — no X11 build; XWayland is not used.

## Quick start

### Requirements

- **Rust 1.99.1+** for building from source.
- `ytrust` 1.1.2+ (1.x series) and `ffmpeg` are required at runtime. Volvid can
  install managed copies from the dependency screen if they are missing. `node`
  20+ is optional and is used to solve YouTube JavaScript challenges for
  ciphered formats.

### Build and run

```bash
git clone https://github.com/VolRencs/Volvid
cd Volvid
cargo build --release -p volvid-gui
./target/release/Volvid
```

Or pass a URL directly:

```bash
./target/release/Volvid 'https://www.youtube.com/watch?v=dQw4w9WgXcQ'
```

### Linux

Volvid targets **Wayland only**. Build and run inside a Wayland session:

```bash
cargo run --release -p volvid-gui
```

### Windows

```powershell
cargo build --release -p volvid-gui
.\target\release\Volvid.exe
```

## Screenshot

![Volvid home](docs/screenshots/gui.png)

## Architecture

```text
crates/
  volvid-core/      pure domain: URL/fragment parsing, profiles, quality, deps, session
  volvid-i18n/      EN/RU strings and formatting
  volvid-engine/    ytrust, ffmpeg, HTTP, dependency installs, cookies, updates, config
  volvid-app/       UI-independent state machine, command/event flow, download queue
  volvid-gui/       GPUI app, theme, pages, SVG icons and thumbnail preview
assets/
  fonts/            Onest with Cyrillic
  icons/            application icon, logo variants, SVG action icons
docs/               GitHub Pages site and screenshots
```

The GUI never calls `ytrust` or `ffmpeg` directly. User actions become
`AppEvent`s; the engine runs on a Tokio runtime and reports progress back to
the UI-independent state machine.

## Runtime paths

| Variable | Role | Default |
|----------|------|---------|
| `VOLVID_CONFIG_DIR` | config | `UserConfigDir/Volvid` |
| `VOLVID_DATA_DIR` | app data | `%LOCALAPPDATA%/Volvid` / `$XDG_DATA_HOME/Volvid` |
| `VOLVID_DEPS_DIR` | managed binaries | `DataDir/deps` |
| `VOLVID_DOWNLOADS_DIR` | fixed download directory | system Downloads |
| `VOLVID_FFMPEG_SHA256` | pin ffmpeg archive checksum | unset |

Configuration is stored in `config.toml`; download history is stored in
`history.json`. The old `.volvid_locale` and `.volvid_downloads_dir` files are
imported once on first launch.

## Development

```bash
cargo fmt --all
cargo clippy --workspace --all-targets -- -D warnings
cargo test --workspace
cargo deny check
cargo run -p volvid-gui
```

Useful environment variables for development:

- `RUST_LOG=volvid_gui=debug,volvid_app=debug` enables UI/state logs.

Live engine checks (require network, ytrust and ffmpeg):

```bash
cargo run -p volvid-engine --example search -- "query"
cargo run -p volvid-engine --example formats            # video/audio/subtitle matrix
cargo run -p volvid-engine --example formats -- --playlist
cargo run -p volvid-engine --example cancel             # process cancellation
```

### Project tooling

- `cargo nextest` for fast test runs (optional).
- `cargo deny check` runs in CI for dependency policy.
- `cargo audit` for additional advisory checks (optional).
- GPUI test support for headless UI checks.

## Platforms

| OS | Arch | Status |
|----|------|--------|
| Windows 10/11 | x86_64 | supported |
| Linux (Wayland) | x86_64 | supported |
| Linux (X11) | x86_64 | not supported |

The Linux build is compiled without GPUI's X11 backend; run it inside a Wayland
session.

## License

Licensed under **GPL-3.0** — see [`LICENSE`](LICENSE).
Copyright (C) 2026 VolRen.

External tools:

- [ytrust](https://github.com/VolRencs/ytrust) — GPL-3.0
- [ffmpeg](https://ffmpeg.org) — LGPL/GPL
- [Node.js](https://nodejs.org) — MIT
- [GPUI](https://github.com/zed-industries/zed) — Apache-2.0
- [Tokio](https://tokio.rs) — MIT
- [Onest](https://github.com/simpals/onest) — OFL
