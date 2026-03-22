# VolRen Video / Audio Downloader

<div align="center">

![youtube downloader screenshot](assets/youtubedownloader.png)

**Download video and audio from YouTube through a terminal UI.**

![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-lightgrey?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Version](https://img.shields.io/badge/Version-5.1.1-orange?style=flat-square)

</div>

---

## About

**VolRen Downloader** is a compact YouTube downloader with an interactive TUI built on [Bubble Tea v2](https://charm.land/bubbletea/v2). It ships as a single Go binary. On first run it pulls **yt-dlp** into `_deps/` (and **ffmpeg** on Windows when you opt in).

The interface is available in **English** and **Russian**. Press **Tab** to switch language; the choice is saved in `.volren_locale` next to the executable.

## Features

- **Best quality** — HD / 4K with stream merge via ffmpeg  
- **Economy** — 360p for slow connections  
- **Audio only** — MP3 with high-quality VBR  
- **Playlists** — browse, toggle with Space, ranges or manual index entry  
- **Parallel downloads** — up to five workers  
- **Format fallback** — tries alternate formats if the first choice fails  
- **Session summary** — per-run download stats  
- **Auto-update** — optional GitHub release check on startup  
- **Dependency updates** — `Ctrl+U` inside the TUI  
- **TUI** — keyboard-driven UI with live progress  

---

## Quick start

### Windows — download the `.exe`

Grab `VolRenDownloader.exe` from the [latest release](https://github.com/VolRencs/YouTubeDownloader/releases/latest) and run it. You do not need Go installed.

### Linux — binary

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

### Build from source

Requires **Go 1.26+**.

```bash
git clone https://github.com/VolRencs/YouTubeDownloader
cd YouTubeDownloader
go mod tidy
go build -ldflags="-s -w" -o VolRenDownloader .
```

On first launch:

1. **yt-dlp** is checked; if missing it is downloaded (~11 MB).  
2. On **Windows**, you may be offered **ffmpeg** (~80 MB).  
3. On **Linux**, ffmpeg is expected on the system (`apt install ffmpeg`, etc.).

---

## Auto-update

On startup the app can check GitHub for a newer release and offer to update.

- **Windows** — new `.exe` plus a `.bat` that replaces the file after you quit.  
- **Linux** — atomic binary replace; restart the app manually.

The `_deps/` and `downloads/` folders are left as-is.

---

## Controls

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Move in menus |
| `Enter` | Confirm |
| `Space` | Toggle item in playlist view |
| `a` / `а` | Select all / clear all |
| `/` | Enter indices manually |
| `Tab` | Switch UI language (EN / RU) |
| `Ctrl+U` | Update yt-dlp (and ffmpeg on Windows) |
| `Ctrl+C` | Quit |

---

## Folder layout

```
next to the binary/
├── VolRenDownloader / VolRenDownloader.exe
├── .volren_locale          ← saved language (en / ru)
├── _deps/                  ← yt-dlp; on Windows also ffmpeg.exe
└── downloads/              ← downloaded files and playlist folders
```

---

## Platforms

| OS | Arch | yt-dlp | ffmpeg | App update |
|----|------|--------|--------|------------|
| Windows | x64 | from GitHub | BtbN build | `.bat` after exit |
| Linux | amd64 / arm64 | from GitHub | system | binary rename |

---

## Cross-compilation

```bash
go mod tidy
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o VolRenDownloader_arm64 .
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o VolRenDownloader.exe .
```

---

## Source layout (5.0)

| File | Role |
|------|------|
| `main.go` | Entry: `signal.NotifyContext`, `tea.NewProgram` |
| `tui/` | Bubble Tea UI: model, events, rendering, styles, custom widgets |
| `internal/app/` | App core: downloads, dependencies, updates, locale strings, playlists |

---

## Troubleshooting

**yt-dlp fails to download** — Check access to `github.com`; you can place the binary in `_deps/` manually.

**YouTube: “Sign in…”** — Update yt-dlp with `Ctrl+U`.

**No ffmpeg (HD / MP3)** — On Linux: `sudo apt install ffmpeg` (or your distro’s package). On Windows, accept the download in the TUI.

**Empty playlist** — The playlist must be public.

---

## Dependencies

| Component | License |
|-----------|---------|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Unlicense |
| [ffmpeg](https://ffmpeg.org) | LGPL/GPL |
| [Bubble Tea v2](https://charm.land/bubbletea/v2), [Lip Gloss](https://charm.land/lipgloss/v2) | MIT |
| `golang.org/x/sys` (Windows) | BSD-3-Clause |

Project source is **MIT**.

---

<div align="center">

Made with ♥ by **VolRen**

</div>
