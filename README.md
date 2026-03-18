# VolRen Video/Audio Downloader

<div align="center">

<pre>
╔══════════════════════════════════════════════════════╗
║         VolRen  Video / Audio  Downloader            ║
║         версия 3.1.0  •  powered by yt-dlp           ║
╚══════════════════════════════════════════════════════╝
</pre>

**Скачивает видео и аудио с YouTube — один бинарник, никаких зависимостей.**

![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-lightgrey?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Version](https://img.shields.io/badge/Version-3.1.0-orange?style=flat-square)

</div>

---

## О проекте

**VolRen Downloader** — загрузчик видео и аудио с YouTube с интерактивным TUI на базе [Bubble Tea](https://github.com/charmbracelet/bubbletea).  
Написан на Go — собирается в один бинарник без рантайма. `yt-dlp` и `ffmpeg` скачиваются автоматически в папку `_deps/` при первом запуске.

---

## Возможности

- 🎬 **Лучшее качество** — HD / 4K, склейка потоков через ffmpeg
- 📱 **Экономичное** — 360p для медленного интернета
- 🎵 **Только аудио** — MP3 с наилучшим VBR-качеством
- 📋 **Плейлисты** — просмотр списка, выбор видео пробелом, диапазонами или вводом
- ⚡ **Параллельная загрузка** — до 5 потоков одновременно
- 🔄 **Цепочка форматов** — автоматически пробует запасной если основной недоступен
- 📊 **Итоги сессии** — статистика всех загрузок за время работы
- 🔔 **Автообновление** — `.exe` проверяет новые версии на GitHub при запуске (только Windows)
- 🖥 **Интерактивный TUI** — навигация стрелками, живой прогресс-бар

---

## Быстрый старт

### Windows — скачать .exe

Скачай `VolRenDownloader.exe` из [последнего релиза](../../releases/latest) и запусти. Go не нужен.

### Linux — скачать бинарник

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

### Собрать из исходников

```bash
git clone https://github.com/VolRencs/YouTubeDownloader
cd YouTubeDownloader
go mod tidy
go build -ldflags="-s -w" -o VolRenDownloader .
```

При первом запуске:
1. Проверяется наличие `yt-dlp` — скачивается если нет (~11 МБ)
2. На Windows предлагается скачать `ffmpeg` (~80 МБ) — нужен для HD и MP3
3. На Linux ffmpeg берётся из системы (`apt install ffmpeg` и т.д.)

---

## Автообновление (только Windows .exe)

При каждом запуске `.exe` автоматически проверяет наличие новой версии на GitHub.  
Если обновление найдено — TUI предложит выбор:

```
  ✔  Доступна новая версия: 3.1.0  (текущая: 3.0.0)

▶ Да
  Нет

  [↑↓] выбрать  [Enter] / [y/n]
```

При согласии скачивает новый `.exe`, запускает `.bat`-скрипт, который заменяет файл после выхода. Папки `_deps/` и `downloads/` не затрагиваются.

---

## Интерфейс

Программа полностью управляется с клавиатуры:

| Клавиша | Действие |
|---|---|
| `↑` / `↓` или `k` / `j` | Навигация по меню |
| `Enter` | Подтвердить выбор |
| `Пробел` | Выбрать / снять видео в плейлисте |
| `a` / `а` | Выбрать все / снять все |
| `/` | Ввести выбор вручную (диапазоны) |
| `y` / `д` | Быстрое «Да» |
| `n` / `н` | Быстрое «Нет» |
| `Ctrl+C` | Выход |

---

## Использование

### Обычное видео

```
  Вставь ссылку:
 ╭──────────────────────────────────────────────────────────────╮
 │ https://youtu.be/dQw4w9WgXcQ                                 │
 ╰──────────────────────────────────────────────────────────────╯

  Выбери качество:

▶ [1]  Лучшее качество (HD / 4K)
  [2]  Экономичное (360p)
  [3]  Только звук (MP3)

  Загружаю…

  Rick Astley - Never Gonna Give You Up
  ↓  [████████████████████░░░░░░░░░░]   62.4%  24.1 МБ/38.7 МБ  3.2 МБ/с
```

### Плейлист

```
  Плейлист: «Lo-Fi Hip Hop Mix»  (47 видео)
  ──────────────────────────────────────────────────────

▶ [✔]    1.  Chilledcow — beats to study/relax to     3:00:14
  [✔]    2.  Lofi Girl — morning vibes                  58:23
  [ ]    3.  College Music — focus mix               1:23:01
  ...

  Выбрано: 2/47

  [↑↓] навигация  [Пробел] выбрать  [a] все  [/] ввод  [Enter] далее
```

**Форматы ручного ввода (`/`):**

| Ввод | Результат |
|---|---|
| `а` или `all` | Все видео |
| `1-10` | С 1 по 10 |
| `1,4,7` | Только №1, №4, №7 |
| `1-3,7,10-12` | Смешанный |

### Параллельная загрузка

```
  Параллельная загрузка:  (47 видео)

  [1]  Последовательно
  [2]  2 потока(ов)
▶ [3]  3 потока(ов)  (рекомендуется)
  [4]  4 потока(ов)
  [5]  5 потока(ов)
```

Прогресс-бар для каждого потока в реальном времени:

```
  Плейлист  ·  47 видео
  ──────────────────────────────────────────────────────

  [1]  Chilledcow — beats to study
       ↓  [████████████░░░░░░░░]   45.2%  12.3 МБ/27.2 МБ  2.1 МБ/с
  [2]  Lofi Girl — morning vibes
       ↓  [████████████████░░░░]   72.8%   8.9 МБ/12.2 МБ  1.8 МБ/с
  [3]  College Music — focus mix
       ⚙  слияние видео+аудио (ffmpeg)…
  ──────────────────────────────────────────────────────
  ✔ 4  ✘ 0  ◷ 40 в очереди  │  4/47
```

### Поддерживаемые ссылки

```
https://www.youtube.com/watch?v=XXXXXXXXXXX      ← обычное видео
https://youtu.be/XXXXXXXXXXX                     ← короткая ссылка
https://www.youtube.com/shorts/XXXXXXXXXXX       ← Shorts
https://www.youtube.com/live/XXXXXXXXXXX         ← прямой эфир / запись
https://www.youtube.com/playlist?list=XXXXX      ← плейлист
https://www.youtube.com/watch?v=XXX&list=XXXXX   ← видео внутри плейлиста
```

---

## Структура папок

```
📁 твоя-папка/
├── VolRenDownloader          ← бинарник (Linux)
├── VolRenDownloader.exe      ← бинарник (Windows)
├── _deps/                    ← зависимости (создаётся автоматически)
│   ├── yt-dlp                ← yt-dlp (Linux)
│   ├── yt-dlp.exe            ← yt-dlp (Windows)
│   └── ffmpeg.exe            ← ffmpeg (Windows)
└── downloads/                ← скачанные файлы
    ├── Название видео.mp4
    ├── Другое видео.mp3
    └── 📁 Название плейлиста/
        ├── 001 - Первое видео.mp4
        └── 002 - Второе видео.mp4
```

---

## Флаги командной строки

| Флаг | Действие |
|---|---|
| *(без флагов)* | Обычный запуск |
| `--update` | Переустановить `yt-dlp` и `ffmpeg` |

> Флаг `--no-autoupdate` удалён в 3.x — автообновление доступно только на Windows и не мешает работе на Linux.

---

## Платформенная поддержка

| ОС | Архитектура | yt-dlp | ffmpeg |
|---|---|---|---|
| Windows | x64 | `yt-dlp.exe` (GitHub) | BtbN build (GitHub, ~80 МБ) |
| Linux | x86_64 | `yt-dlp_linux` (GitHub) | системный (`apt`/`dnf`/`pacman`) |
| Linux | arm64 / aarch64 | `yt-dlp_linux_aarch64` (GitHub) | системный |

---

## Сборка и разработка

```bash
# Зависимости
go mod tidy

# Linux
go build -ldflags="-s -w" -o VolRenDownloader .

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o VolRenDownloader.exe .

# Linux arm64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o VolRenDownloader_arm64 .
```

**Структура кода:**

| Файл | Назначение |
|---|---|
| `core.go` | Логика: загрузка, плейлисты, зависимости, автообновление |
| `main.go` | Bubble Tea TUI: модель, экраны, клавиши, отображение |
| `platform_windows.go` | Windows: ANSI-консоль, DETACHED_PROCESS для .bat |
| `platform_unix.go` | Linux/macOS: Setsid для фонового процесса |

---

## Устранение проблем

**`yt-dlp` не скачивается**  
Проверь доступность `github.com`. Можно скачать вручную в `_deps/`: [github.com/yt-dlp/yt-dlp/releases](https://github.com/yt-dlp/yt-dlp/releases).

**Ошибка `Sign in to confirm you're not a bot`**  
YouTube блокирует старые версии yt-dlp. Обнови зависимости:
```bash
./VolRenDownloader --update
```

**HD и MP3 недоступны (нет ffmpeg)**  
На Linux установи ffmpeg через пакетный менеджер:
```bash
sudo apt install ffmpeg      # Debian / Ubuntu
sudo dnf install ffmpeg      # Fedora
sudo pacman -S ffmpeg        # Arch
```
На Windows программа предложит скачать ffmpeg при первом запуске.

**Плейлист не загружается**  
Плейлист должен быть публичным — приватные и закрытые недоступны без авторизации.

**Автообновление не срабатывает**  
Автообновление работает только для `.exe` на Windows. На Linux обнови бинарник вручную из [releases](../../releases/latest).

---

## Зависимости и лицензии

| Компонент | Лицензия | Источник |
|---|---|---|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Unlicense | github.com/yt-dlp/yt-dlp |
| [ffmpeg](https://ffmpeg.org) | LGPL 2.1+ / GPL 2+ | ffmpeg.org |
| [BtbN FFmpeg Builds](https://github.com/BtbN/FFmpeg-Builds) | GPL | github.com/BtbN |
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | MIT | github.com/charmbracelet |
| [Bubbles](https://github.com/charmbracelet/bubbles) | MIT | github.com/charmbracelet |
| [Lip Gloss](https://github.com/charmbracelet/lipgloss) | MIT | github.com/charmbracelet |

Исходный код — **MIT License**.

---

<div align="center">

Сделано с ♥ by **VolRen**

</div>
