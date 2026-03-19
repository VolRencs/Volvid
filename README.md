# VolRen Video/Audio Downloader

<div align="center">

<pre>
    ╭────────────────────────────────────────────╮
    │    VolRen  ·  Video / Audio  Downloader    │
    │    версия 4.0.1  •  powered by yt-dlp      │
    ╰────────────────────────────────────────────╯
</pre>

**Скачивает видео и аудио с YouTube — один бинарник, никаких зависимостей.**

![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-lightgrey?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Version](https://img.shields.io/badge/Version-4.0.1-orange?style=flat-square)

</div>

---

## О проекте

**VolRen Downloader** — загрузчик видео и аудио с YouTube с интерактивным TUI на базе [Bubble Tea v2](https://charm.land/bubbletea/v2).  
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
- 🔔 **Автообновление** — проверяет новые версии на GitHub при запуске
- 🔧 **Обновление зависимостей** — `Ctrl+U` в любой момент из TUI
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

## Автообновление

При каждом запуске автоматически проверяет наличие новой версии на GitHub.  
Если обновление найдено — TUI предложит выбор:

```
  ✔  Доступна версия 4.0.1  (сейчас 4.0.0)

▶ Да, обновить
  Пропустить

  [↑↓] выбрать  [Enter] выбрать
```

**Windows** — скачивает новый `.exe`, запускает `.bat`-скрипт, который заменяет файл после выхода программы.  
**Linux** — заменяет бинарник атомарно через `rename(2)`. Просто перезапустите программу после обновления.  

Папки `_deps/` и `downloads/` не затрагиваются.

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
| `Ctrl+U` | Обновить зависимости (yt-dlp, ffmpeg) |
| `Ctrl+C` | Выход |

---

## Использование

### Обычное видео

```
  Вставь ссылку на видео или плейлист

 ╭──────────────────────────────────────────────────────────────╮
 │ https://youtu.be/dQw4w9WgXcQ                                 │
 ╰──────────────────────────────────────────────────────────────╯

  Выбери качество:

▶ ▲ Лучшее качество (HD·4K)
  ▼ Экономичное (360p)
  ♪ Только аудио (MP3)

  Загружаю…

  Rick Astley - Never Gonna Give You Up
  ↓  [████████████████████░░░░░░░░░░]   62.4%  24.1 МБ/38.7 МБ  3.2 МБ/с
```

### Плейлист

```
  Плейлист: «Lo-Fi Hip Hop Mix»  (47 видео)
  ──────────────────────────────────────────────────────

▶ [✔]    1.  Chilledcow — beats to study/relax to     3:00:14
  [✔]    2.  Lofi Girl — morning vibes                58:23
  [ ]    3.  College Music — focus mix                1:23:01
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

▶ Последовательно (1 поток)
  2 потоков
  3 потоков
  4 потоков
  5 потоков
```

Прогресс-бар для каждого потока в реальном времени:

```
  Плейлист  ·  47 видео
  [████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]  17.0%  8/47

  ● [1]  Chilledcow — beats to study
         [████████████░░░░░░░░]   45.2%  12.3 МБ/27.2 МБ  2.1 МБ/с
  ● [2]  Lofi Girl — morning vibes
         [████████████████░░░░]   72.8%   8.9 МБ/12.2 МБ  1.8 МБ/с
  ● [3]  College Music — focus mix
         ⚙ слияние видео+аудио (ffmpeg)…

  ✔ 4  ✘ 0  ◷ 40 в очереди  00:42
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

## Платформенная поддержка

| ОС | Архитектура | yt-dlp | ffmpeg | Автообновление |
|---|---|---|---|---|
| Windows | x64 | `yt-dlp.exe` (GitHub) | BtbN build (GitHub, ~80 МБ) | ✔ bat-скрипт |
| Linux | x86_64 | `yt-dlp_linux` (GitHub) | системный (`apt`/`dnf`/`pacman`) | ✔ atomic rename |
| Linux | arm64 | `yt-dlp_linux_aarch64` (GitHub) | системный | ✔ atomic rename |

---

## Сборка и разработка

```bash
# Зависимости
go mod tidy

# Linux amd64
go build -ldflags="-s -w" -o VolRenDownloader .

# Linux arm64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o VolRenDownloader_arm64 .

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o VolRenDownloader.exe .
```

**Структура кода:**

| Файл | Назначение |
|---|---|
| `core.go` | Логика: загрузка, плейлисты, зависимости, автообновление |
| `main.go` | Bubble Tea v2 TUI: модель, экраны, клавиши, отображение |
| `platform_windows.go` | Windows: ANSI-консоль через `golang.org/x/sys/windows`, DETACHED_PROCESS |
| `platform_unix.go` | Linux: Setsid, атомарная замена бинарника |

---

## Устранение проблем

**`yt-dlp` не скачивается**  
Проверь доступность `github.com`. Можно скачать вручную в `_deps/`: [github.com/yt-dlp/yt-dlp/releases](https://github.com/yt-dlp/yt-dlp/releases).

**Ошибка `Sign in to confirm you're not a bot`**  
YouTube блокирует старые версии yt-dlp. Обнови зависимости через `Ctrl+U` прямо в TUI.

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

---

## Зависимости и лицензии

| Компонент | Версия | Лицензия | Источник |
|---|---|---|---|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | latest | Unlicense | github.com/yt-dlp/yt-dlp |
| [ffmpeg](https://ffmpeg.org) | latest | LGPL 2.1+ / GPL 2+ | ffmpeg.org |
| [BtbN FFmpeg Builds](https://github.com/BtbN/FFmpeg-Builds) | latest | GPL | github.com/BtbN |
| [Bubble Tea](https://charm.land/bubbletea/v2) | v2 | MIT | charm.land/bubbletea/v2 |

Исходный код — **MIT License**.

---

<div align="center">

Сделано с ♥ by **VolRen**

</div>