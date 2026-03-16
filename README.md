# VolRen Video/Audio Downloader

<div align="center">

```
╔══════════════════════════════════════════════════════╗
║         VolRen  Video / Audio  Downloader            ║
║         версия 1.0.0  •  powered by yt-dlp           ║
╚══════════════════════════════════════════════════════╝
```

**Скачивает видео и аудио с YouTube — без pip, без root, без лишних движений.**

![Python](https://img.shields.io/badge/Python-3.10%2B-blue?style=flat-square&logo=python)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-lightgrey?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Version](https://img.shields.io/badge/Version-1.0.0-orange?style=flat-square)

</div>

---

## О проекте

**VolRen Downloader** — консольный загрузчик видео и аудио с YouTube.  
Все зависимости (`yt-dlp`, `ffmpeg`) скачиваются автоматически в папку `_deps/` рядом со скриптом при первом запуске. Ничего не устанавливается в систему, `pip` не нужен.

---

## Возможности

- 🎬 **Лучшее качество** — HD / 4K, склейка видео и аудио потоков через ffmpeg
- 📱 **Экономичное качество** — 360p для медленного интернета или мобильных
- 🎵 **Только аудио** — MP3 с наилучшим VBR-качеством
- 📋 **Плейлисты** — просмотр списка, выбор конкретных видео, диапазонов или всего сразу
- 🔄 **Цепочка форматов** — если основной формат недоступен, автоматически пробуется запасной
- 📊 **Итоги сессии** — статистика успешных и неудачных загрузок за всё время работы
- 🔁 **Режим нескольких загрузок** — после каждого скачивания предлагает продолжить
- 🛠 **Автообновление** — флаг `--update` перекачивает все зависимости на последние версии

---

## Требования

| Что | Версия |
|---|---|
| Python | 3.10 или новее |
| yt-dlp | скачивается автоматически |
| ffmpeg | скачивается автоматически |
| pip | **не нужен** |
| root / sudo | **не нужен** |

---

## Быстрый старт

```bash
# 1. Скопируй скрипт в любую папку
# 2. Запусти — всё остальное сделается само

python VolRenDownloader.py
```

При первом запуске скрипт:
1. Проверит версию Python
2. Скачает `yt-dlp` (~11 МБ) прямо с GitHub
3. Предложит скачать `ffmpeg` (~80 МБ) — нужен для HD и MP3
4. Спросит ссылку и качество

---

## Структура папок

```
📁 твоя-папка/
├── VolRenDownloader.py      ← скрипт
├── _deps/                   ← зависимости (создаётся автоматически)
│   ├── yt-dlp               ← бинарник yt-dlp (Linux)
│   ├── yt-dlp.exe           ← бинарник yt-dlp (Windows)
│   ├── ffmpeg               ← бинарник ffmpeg (Linux)
│   └── ffmpeg.exe           ← бинарник ffmpeg (Windows)
└── downloads/               ← все скачанные файлы
    ├── Название видео.mp4
    ├── Другое видео.mp3
    └── 📁 Название плейлиста/
        ├── 001 - Первое видео.mp4
        └── 002 - Второе видео.mp4
```

---

## Использование

### Обычное видео

```
  Ссылка на видео: https://youtu.be/dQw4w9WgXcQ

  Выбери качество:
  1  — Лучшее качество (HD / 4K)
  2  — Экономичное (360p)
  3  — Только звук (MP3)

  Твой выбор [1/2/3]: 1

  ────────────────────────────────────────────────────────
  →  Скачиваю в лучшем качестве…
  ✔  Готово! → /home/user/videos/downloads
  ────────────────────────────────────────────────────────
  
  Скачать ещё? [д] / [н]
```

### Плейлист

Когда скрипт получает ссылку на плейлист, он показывает список всех видео и предлагает выбрать что именно скачать:

```
  →  Получаю информацию о плейлисте…

  Плейлист: «Lo-Fi Hip Hop Mix»  (47 видео)
  ────────────────────────────────────────────────────────
     1.  Chilledcow — beats to study/relax to         3:00:14
     2.  Lofi Girl — morning vibes                      58:23
     3.  College Music — focus mix                   1:23:01
     ...

  Что скачать?
  а   — Все 47 видео
  1-5 — Диапазон номеров
  1,3 — Конкретные номера
  Смесь: 1-3,7,10-12

  Выбор: 1-3,7
```

**Форматы выбора для плейлиста:**

| Ввод | Результат |
|---|---|
| `а` или `all` | Все видео в плейлисте |
| `1-10` | Видео с 1 по 10 |
| `1,4,7` | Только видео №1, №4 и №7 |
| `1-3,7,10-12` | Смешанный выбор |

### Поддерживаемые форматы ссылок

```
https://www.youtube.com/watch?v=XXXXXXXXXXX      ← обычное видео
https://youtu.be/XXXXXXXXXXX                     ← короткая ссылка
https://www.youtube.com/shorts/XXXXXXXXXXX       ← Shorts
https://www.youtube.com/live/XXXXXXXXXXX         ← прямой эфир / запись
https://www.youtube.com/playlist?list=XXXXX      ← плейлист
https://www.youtube.com/watch?v=XXX&list=XXXXX   ← видео внутри плейлиста
```

---

## Итоги сессии

После завершения работы скрипт показывает сводку всех загрузок:

```
  Итоги сессии:
  ────────────────────────────────────────────────────────
  [OK  ]  Лучшее качество    https://youtu.be/dQw4w9WgXcQ
  [OK  ]  MP3                https://youtu.be/abc123XXXXX
  [FAIL]  360p               https://youtu.be/unavailable…
  ────────────────────────────────────────────────────────
  Всего: 3  ·  Успешно: 2  ·  Ошибок: 1
```

---

## Обновление зависимостей

```bash
python VolRenDownloader.py --update
```

Перекачает `yt-dlp` и `ffmpeg` на последние версии.  
Рекомендуется запускать раз в месяц — YouTube регулярно меняет API.

---

## Платформенная поддержка

| ОС | Архитектура | yt-dlp | ffmpeg |
|---|---|---|---|
| Windows | x64 | `yt-dlp.exe` (GitHub) | BtbN build (GitHub) |
| Linux | x86_64 | `yt-dlp_linux` (GitHub) | johnvansickle static |
| Linux | arm64 / aarch64 | `yt-dlp_linux_aarch64` (GitHub) | johnvansickle static |

---

## Устранение проблем

**`yt-dlp` не скачивается**  
Проверь интернет-соединение. Скрипт качает бинарник с `github.com` — убедись, что он доступен.  
Можно скачать вручную: [github.com/yt-dlp/yt-dlp/releases](https://github.com/yt-dlp/yt-dlp/releases) и положить в `_deps/`.

**Ошибка `Sign in to confirm you're not a bot`**  
YouTube периодически блокирует старые версии yt-dlp. Запусти обновление:
```bash
python VolRenDownloader.py --update
```

**Режимы 1 и 3 недоступны (нет ffmpeg)**  
ffmpeg нужен для склейки видео+аудио и конвертации в MP3.  
Скрипт предложит скачать его при запуске — ответь `д`.

**Видео скачивается без звука**  
Это происходит когда ffmpeg не найден, а YouTube отдаёт видео и аудио отдельными потоками.  
Установи ffmpeg через предложение при запуске.

**Плейлист не загружается**  
Убедись, что плейлист публичный. Приватные и закрытые плейлисты недоступны без авторизации.

---

## Зависимости и лицензии

| Компонент | Лицензия | Источник |
|---|---|---|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Unlicense | github.com/yt-dlp/yt-dlp |
| [ffmpeg](https://ffmpeg.org) | LGPL 2.1+ / GPL 2+ | ffmpeg.org |
| [BtbN FFmpeg Builds](https://github.com/BtbN/FFmpeg-Builds) | GPL | github.com/BtbN |
| [johnvansickle ffmpeg](https://johnvansickle.com/ffmpeg/) | GPL | johnvansickle.com |

Сам скрипт `VolRenDownloader.py` — **MIT License**.


<div align="center">

Сделано с ♥ by **VolRen**

</div>
