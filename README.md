# VolRen Video / Audio Downloader

<div align="center">

![downloader](assets/Downloader.png)

**Download video and audio from YouTube through a terminal UI or Telegram bot.**

![Go](https://img.shields.io/badge/Go-1.26.1%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-lightgrey?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Version](https://img.shields.io/badge/Version-6.2.1-orange?style=flat-square)

</div>

---

## About

![youtube downloader screenshot](assets/youtubedownloader.png)

**VolRen Downloader** is a compact YouTube downloader built around **yt-dlp**.

The project includes:

- a keyboard-driven **stage-based TUI** built on [Bubble Tea v2](https://charm.land/bubbletea/v2)
- an optional **Telegram bot** for downloading by URL or by YouTube search

The app checks **yt-dlp** and **ffmpeg** on startup and opens a dependency screen if one of them is missing. Managed binaries can be downloaded into `_deps/`, while system-installed binaries are preferred when available. **node** is optional and is used as a **yt-dlp JS runtime** when found. Browser cookies are auto-detected from supported browsers on **Linux** and **Windows**. The TUI uses a clean centered card with aligned menus, consistent notices, flat key hints, and **English / Russian** localization via **Tab**.

---

## Features

### TUI

- **Unified staged flow**: update check, dependencies, target, search, playlist, profile, download, summary
- **Clean single-card layout** with aligned menus, consistent notices, progress blocks and footer hints
- **Best / Economy video presets** with quality scan via yt-dlp
- **Audio-only mode** with 5 presets: MP3 320k, MP3 192k, M4A/AAC Best, Opus Best, FLAC
- **Thumbnail download**
- **Playlist browser** with Space selection, manual ranges and multi-worker downloads
- **YouTube search** from the main input via `Ctrl+G`
- **Auto-update** check on startup
- **Dependency screen** with `yt-dlp`, `ffmpeg`, `node`, browser cookies and JS runtime status
- **Managed dependency refresh** inside the UI with `Ctrl+U`
- **Downloads folder quick-open** from the main URL screen and the session summary
- **Session summary** with per-run history and a quick open-folder action

### Telegram bot

- **Direct YouTube URL** flow for video, audio, thumbnail and selected playlist videos
- **Text search**: send a video title, get 3-5 YouTube results, choose with buttons
- **Playlist selection limits**:
  - regular users: up to **5 videos**
  - premium users: up to **30 videos**
- **Download limits**:
  - regular users: **500 MB**
  - premium users: **2 GB**
- **Double size checks** for the bot:
  - estimate before download
  - actual user job-folder size after download
- **Premium via Telegram Stars**
- **Admin tools**:
  - `/broadcast`
  - `/schedule`
  - `/timers`
  - `/deltimer`
  - `/status`
  - `/update`

---

## Quick Start

### Windows

Download `VolRenDownloader.exe` from the [latest release](https://github.com/VolRencs/YouTubeDownloader/releases/latest) and run it. You do not need Go installed.

### Linux

```bash
# amd64
curl -L https://github.com/VolRencs/YouTubeDownloader/releases/latest/download/VolRenDownloader_linux_amd64 -o VolRenDownloader
chmod +x VolRenDownloader
./VolRenDownloader

# arm64
curl -L https://github.com/VolRencs/YouTubeDownloader/releases/latest/download/VolRenDownloader_linux_arm64 -o VolRenDownloader
chmod +x VolRenDownloader
./VolRenDownloader
```

### Build The TUI From Source

Requires **Go 1.26.1+**.

```bash
git clone https://github.com/VolRencs/YouTubeDownloader
cd YouTubeDownloader
go build -trimpath -buildvcs=false -ldflags="-s -w" -o VolRenDownloader ./cmd/downloader
./VolRenDownloader
```

### Build The Windows TUI With Icon

The repository includes a Windows icon source at `assets/icon/icon.ico`.

```bash
go install github.com/akavel/rsrc@latest
GOARCH=amd64 ./scripts/build-windows-downloader.sh VolRenDownloader.exe
```

### Build The Telegram Bot

Configure the bot through environment variables or an optional JSON config file.

Environment variables:

- `VOLREN_BOT_TOKEN`
- `VOLREN_BOT_LOCAL_SERVER`
- `VOLREN_BOT_API_URL`
- `VOLREN_BOT_ADMIN_IDS`
- `VOLREN_BOT_OWNER_IDS`
- `VOLREN_BOT_PREMIUM_STARS_PRICE`
- `VOLREN_BOT_CONFIG` for an optional JSON config path

Minimal example:

```bash
export VOLREN_BOT_TOKEN="123456:token"
export VOLREN_BOT_LOCAL_SERVER=true
export VOLREN_BOT_API_URL="http://127.0.0.1:8081"
export VOLREN_BOT_ADMIN_IDS="123456789"
export VOLREN_BOT_OWNER_IDS="123456789"
```

Then build the bot with the `bot` build tag:

```bash
go build -tags bot -trimpath -buildvcs=false -ldflags="-s -w" -o tgbot ./cmd/tgbot
./tgbot
```

---

## TUI Flow

1. Start in the **update / dependency** stage.
2. Paste a YouTube link on the **target** screen or press `Ctrl+G` to search.
3. If the URL is a playlist, choose items with `Space`, `a` or `/`.
4. Choose **Video / Audio / Thumbnail**.
5. For video: choose quality. For audio: choose one of the available presets.
6. Watch progress in the unified **download** stage, then use **summary** to open the downloads folder with `O` or return to the URL screen. You can also open the downloads folder from the main URL screen with `O`.

On startup:

1. `yt-dlp` and `ffmpeg` are treated as required dependencies.
2. If one of them is missing, the TUI opens the dependency screen before the URL screen.
3. `node` is optional and can be downloaded from the same screen, but it does not block the app.
4. Browser cookies and JS runtime status are detected automatically.

---

## Telegram Bot Commands

### User commands

- send a **YouTube URL** to start the download flow
- for playlists, choose specific videos instead of downloading the whole playlist
- send plain **text** to search YouTube
- `/premium` to buy lifetime premium with Telegram Stars
- `/cancel` to stop the current flow

### Admin commands

- `/status` show dependency, browser cookies, JS runtime and Bot API backend status
- `/update` refresh managed runtime dependencies
- `/broadcast <text>` or reply with `/broadcast`
- `/schedule <duration> <text>` or reply with `/schedule <duration>`
- `/timers`
- `/deltimer <id>`

Notes:

- Bot playlists are downloaded only through explicit video selection.
- Regular users can choose up to **5 videos** from a playlist.
- Premium users can choose up to **30 videos** from a playlist.
- Regular users are limited to **500 MB** per download job.
- Premium users are limited to **2 GB** per download job.
- For playlists, the bot checks the **whole user job folder** size.
- `premium_users.json` and `bot_users.json` are reloaded automatically on the next access.
- `bot_timers.json` is synchronized in the background, usually within about **1 second**.
- Telegram delivery still depends on the selected Bot API backend:
  - cloud Bot API: up to **50 MB**
  - local Bot API server: up to **2000 MB**

---

## Controls

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move in menus |
| `Enter` | Continue |
| `Space` | Select item in playlist view |
| `/` | Enter playlist indices manually |
| `Ctrl+G` | Open YouTube search from the main URL screen |
| `Esc` | Leave search and return to URL input |
| `a` / `а` | Select all / clear all in playlists |
| `Tab` | Switch UI language (EN / RU) |
| `Ctrl+U` | Open dependency management / update managed dependencies |
| `O` | Open the downloads folder from the main URL or summary screen |

---

## Platforms

| OS | Arch | yt-dlp | ffmpeg | node | App update |
|----|------|--------|--------|------|------------|
| Windows | x64 / arm64 | system or `_deps` | system or `_deps` | system or `_deps` | `.bat` replace after close |
| Linux | amd64 / arm64 | system or `_deps` | system or `_deps` | system or `_deps` | binary replace |

---

## Cross-Compilation

```bash
GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags="-s -w" -o VolRenDownloader_arm64 ./cmd/downloader
go install github.com/akavel/rsrc@latest
GOARCH=amd64 ./scripts/build-windows-downloader.sh VolRenDownloader.exe
```

---

## Source Layout

| Path | Role |
|------|------|
| `cmd/downloader/` | TUI entrypoint |
| `cmd/tgbot/` | Telegram bot entrypoint |
| `tui/` | Bubble Tea model, events, rendering, search flow and widgets |
| `internal/app/` | shared runtime helpers: downloads, search, deps, updates, locale, playlists, HTTP client |
| `internal/bot/` | bot routing, sessions, premium, storage, scheduler, admin tools and Telegram API helpers |

---

## Troubleshooting

**`go build ./cmd/tgbot` fails**  
Use `-tags bot`. The bot entrypoint is guarded by a build tag:

```bash
go build -tags bot ./cmd/tgbot
```

**yt-dlp fails to download**  
Check access to GitHub or place the binary into `_deps/` manually.

**YouTube says “Sign in to confirm you’re not a bot”**  
Check the dependency screen or `/status` first. The app auto-detects browser cookies and JS runtime; if cookies are inactive, make sure you have a supported browser profile on the same machine.

**HD merge / MP3 conversion fails**  
Make sure `ffmpeg` is available. If it is missing, open the dependency screen in the TUI or prepare it in `_deps/`. System-installed `ffmpeg` is preferred when available.

**Large bot downloads are rejected**  
Check both limits:

- app-side download limit: `500 MB` or `2 GB` for premium
- Telegram delivery backend limit: `50 MB` cloud or `2000 MB` local Bot API server

**Manual JSON edits do not seem to apply**  
`premium_users.json`, `bot_users.json` and `bot_timers.json` are supported as live runtime state. Keep them valid JSON if you edit them by hand:

- premium and known users are reloaded on the next store access
- timers are synchronized in the background, usually within about `1s`

---

## Dependencies

| Component | License |
|-----------|---------|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Unlicense |
| [ffmpeg](https://ffmpeg.org) | LGPL/GPL |
| [Bubble Tea v2](https://charm.land/bubbletea/v2) | MIT |
| [Lip Gloss v2](https://charm.land/lipgloss/v2) | MIT |
| [go-telegram/bot](https://github.com/go-telegram/bot) | MIT |

Project source is **MIT**.

---

<div align="center">

Made with ♥ by **VolRen**

</div>
