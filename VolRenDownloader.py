"""
VolRen Video/Audio Downloader  —  версия 1.0.0
Автор : VolRen
Инфо  : Все зависимости (ffmpeg, yt-dlp) скачиваются автоматически
        в папку _deps/ рядом со скриптом. Ничего не устанавливается
        в систему. Работает на Windows и Linux (x64 / arm64).
Нужно : Python 3.10+
"""

from __future__ import annotations

import os
import platform
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from dataclasses import dataclass, field
from pathlib import Path

# ══════════════════════════════════════════════════════════════════════════════
#  КОНСТАНТЫ
# ══════════════════════════════════════════════════════════════════════════════

VERSION    = "1.0.0"
SCRIPT_DIR = Path(__file__).resolve().parent
DEPS_DIR   = SCRIPT_DIR / "_deps"          # все зависимости сюда
DL_DIR     = SCRIPT_DIR / "downloads"      # сюда сохраняются видео/аудио
OUTPUT_TMPL          = str(DL_DIR / "%(title)s.%(ext)s")
OUTPUT_TMPL_PLAYLIST = str(DL_DIR / "%(playlist_title)s" / "%(playlist_index)s - %(title)s.%(ext)s")

IS_WINDOWS = sys.platform == "win32"
ARCH       = platform.machine().lower()    # amd64 / x86_64 / aarch64 / arm64

# ─── URL ffmpeg ────────────────────────────────────────────────────────────────

_BTBN = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/"

def _ffmpeg_url() -> str:
    if IS_WINDOWS:
        return _BTBN + "ffmpeg-master-latest-win64-gpl.zip"
    if ARCH in ("aarch64", "arm64"):
        return _BTBN + "ffmpeg-master-latest-linuxarm64-gpl.tar.xz"
    return _BTBN + "ffmpeg-master-latest-linux64-gpl.tar.xz"

FFMPEG_BIN  = DEPS_DIR / ("ffmpeg.exe"  if IS_WINDOWS else "ffmpeg")
YTDLP_BIN   = DEPS_DIR / ("yt-dlp.exe" if IS_WINDOWS else "yt-dlp")

# URL standalone-бинарника yt-dlp
def _ytdlp_url() -> str:
    if IS_WINDOWS:
        return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
    if ARCH in ("aarch64", "arm64"):
        return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64"
    return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux"

# Паттерн YouTube-ссылок (видео, shorts, live, плейлисты)
_YT_RE = re.compile(
    r"(youtube\.com/(watch\?.*v=|shorts/|live/|playlist\?list=)|youtu\.be/)[\w\-]{1,}",
    re.IGNORECASE,
)

# Паттерн для определения что ссылка — плейлист
_PLAYLIST_RE = re.compile(
    r"youtube\.com/(playlist\?|.*[?&]list=)[\w\-]{10,}",
    re.IGNORECASE,
)

# ══════════════════════════════════════════════════════════════════════════════
#  ЦВЕТА
# ══════════════════════════════════════════════════════════════════════════════

def _enable_ansi() -> None:
    """Включает ANSI-escape на Windows через виртуальный терминал."""
    if IS_WINDOWS:
        os.system("")          # простейший способ включить VT в cmd/powershell


_enable_ansi()


class C:
    RESET   = "\033[0m"
    BOLD    = "\033[1m"
    RED     = "\033[91m"
    GREEN   = "\033[92m"
    YELLOW  = "\033[93m"
    CYAN    = "\033[96m"
    MAGENTA = "\033[95m"
    GRAY    = "\033[90m"
    WHITE   = "\033[97m"


def log_ok(msg: str)   -> None: print(f"{C.GREEN}  ✔  {msg}{C.RESET}")
def log_err(msg: str)  -> None: print(f"{C.RED}  ✘  {msg}{C.RESET}")
def log_info(msg: str) -> None: print(f"{C.CYAN}  →  {msg}{C.RESET}")
def log_warn(msg: str) -> None: print(f"{C.YELLOW}  !  {msg}{C.RESET}")
def log_sep()          -> None: print(f"{C.GRAY}{'─' * 56}{C.RESET}")


BANNER = f"""{C.CYAN}
 ╔══════════════════════════════════════════════════════╗
 ║         VolRen  Video / Audio  Downloader            ║
 ║         версия {VERSION}  •  powered by yt-dlp           ║
 ╚══════════════════════════════════════════════════════╝
{C.RESET}"""

# ══════════════════════════════════════════════════════════════════════════════
#  ПРОГРЕСС-БАР ДЛЯ СКАЧИВАНИЯ
# ══════════════════════════════════════════════════════════════════════════════

def _progress_hook(block_num: int, block_size: int, total_size: int) -> None:
    """Reporthook для urllib.request.urlretrieve — рисует прогресс в одну строку."""
    if total_size <= 0:
        downloaded = block_num * block_size
        print(f"\r  {C.CYAN}↓{C.RESET}  {downloaded / 1_048_576:.1f} МБ…", end="", flush=True)
        return
    done     = min(block_num * block_size, total_size)
    pct      = done / total_size
    bar_len  = 30
    filled   = int(bar_len * pct)
    bar      = "█" * filled + "░" * (bar_len - filled)
    mb_done  = done / 1_048_576
    mb_total = total_size / 1_048_576
    print(
        f"\r  {C.CYAN}↓{C.RESET}  [{bar}] {pct*100:5.1f}%  "
        f"{mb_done:.1f}/{mb_total:.1f} МБ",
        end="",
        flush=True,
    )


def _download_file(url: str, dest: Path) -> None:
    """Скачивает файл по URL в dest, показывая прогресс."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    urllib.request.urlretrieve(url, dest, reporthook=_progress_hook)
    print()   # перевод строки после прогресс-бара


# ══════════════════════════════════════════════════════════════════════════════
#  УСТАНОВКА ffmpeg В _deps/
# ══════════════════════════════════════════════════════════════════════════════

def _extract_ffmpeg_from_zip(archive: Path) -> None:
    """Извлекает ffmpeg.exe из Windows zip-архива BtbN."""
    with zipfile.ZipFile(archive) as zf:
        for member in zf.namelist():
            name = Path(member).name
            if name in ("ffmpeg.exe", "ffprobe.exe"):
                data = zf.read(member)
                out  = DEPS_DIR / name
                out.write_bytes(data)
                log_ok(f"Извлечён: {name}")


def _extract_ffmpeg_from_tar(archive: Path) -> None:
    """Извлекает ffmpeg/ffprobe из Linux static tar.xz (BtbN builds)."""
    with tarfile.open(archive, "r:xz") as tf:
        for member in tf.getmembers():
            name = Path(member.name).name
            if name in ("ffmpeg", "ffprobe") and member.isfile():
                member.name = name          # сбрасываем путь — кладём плоско
                tf.extract(member, path=DEPS_DIR)
                bin_path = DEPS_DIR / name
                # Даём права на исполнение
                bin_path.chmod(bin_path.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
                log_ok(f"Извлечён: {name}")


def install_ffmpeg() -> None:
    """Скачивает и распаковывает ffmpeg в DEPS_DIR."""
    DEPS_DIR.mkdir(parents=True, exist_ok=True)
    url = _ffmpeg_url()
    ext = ".zip" if IS_WINDOWS else ".tar.xz"

    with tempfile.TemporaryDirectory() as tmp:
        archive = Path(tmp) / f"ffmpeg{ext}"
        log_info(f"Скачиваю ffmpeg ({ARCH}, {'Windows' if IS_WINDOWS else 'Linux'})…")
        try:
            _download_file(url, archive)
        except Exception as exc:
            log_err(f"Ошибка скачивания ffmpeg: {exc}")
            log_err("Проверь интернет-соединение и попробуй снова.")
            sys.exit(1)

        log_info("Распаковываю…")
        if IS_WINDOWS:
            _extract_ffmpeg_from_zip(archive)
        else:
            _extract_ffmpeg_from_tar(archive)

    log_ok(f"ffmpeg установлен в: {DEPS_DIR}")


# ══════════════════════════════════════════════════════════════════════════════
#  УСТАНОВКА yt-dlp В _deps/
# ══════════════════════════════════════════════════════════════════════════════

def _ytdlp_ready() -> bool:
    """True если бинарник yt-dlp уже есть и исполняем."""
    if not YTDLP_BIN.exists():
        return False
    try:
        r = subprocess.run([str(YTDLP_BIN), "--version"],
                           capture_output=True, timeout=10)
        return r.returncode == 0
    except Exception:
        return False


def install_yt_dlp() -> None:
    """
    Скачивает standalone-бинарник yt-dlp прямо с GitHub releases.
    Не требует pip, не требует прав root, не трогает системный Python.
    """
    DEPS_DIR.mkdir(parents=True, exist_ok=True)
    url = _ytdlp_url()
    log_info(f"Скачиваю yt-dlp ({ARCH}, {'Windows' if IS_WINDOWS else 'Linux'})…")
    log_info(f"Источник: {url}")
    try:
        _download_file(url, YTDLP_BIN)
    except Exception as exc:
        log_err(f"Ошибка скачивания yt-dlp: {exc}")
        log_err("Проверь интернет-соединение и попробуй снова.")
        sys.exit(1)

    # На Linux/macOS — ставим бит исполнения
    if not IS_WINDOWS:
        YTDLP_BIN.chmod(
            YTDLP_BIN.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH
        )

    if _ytdlp_ready():
        log_ok(f"yt-dlp готов: {YTDLP_BIN.name}")
    else:
        log_err("Бинарник скачан, но не запускается. Попробуй скачать вручную:")
        log_err(f"  {url}")
        log_err(f"  → положи файл в {DEPS_DIR}")
        sys.exit(1)


# ══════════════════════════════════════════════════════════════════════════════
#  ПРОВЕРКИ ОКРУЖЕНИЯ
# ══════════════════════════════════════════════════════════════════════════════

FFMPEG_RESOLVED: str | None = None   # заполняется в run_checks()


def check_python() -> None:
    major, minor = sys.version_info[:2]
    if (major, minor) < (3, 10):
        log_err(f"Python {major}.{minor} не поддерживается. Нужен 3.10+.")
        sys.exit(1)
    log_ok(f"Python {major}.{minor}  ({sys.platform} / {ARCH})")


def check_ffmpeg() -> None:
    global FFMPEG_RESOLVED
    if FFMPEG_BIN.exists():
        FFMPEG_RESOLVED = str(FFMPEG_BIN)
        log_ok(f"ffmpeg найден в _deps/")
        return
    # Предлагаем скачать
    log_warn("ffmpeg не найден в _deps/.")
    ans = _ask(f"  {C.BOLD}Скачать ffmpeg автоматически?{C.RESET} {C.CYAN}[д]{C.RESET}/{C.RED}[н]{C.RESET}  ").lower()
    if ans in ("д", "да", "y", "yes", ""):
        install_ffmpeg()
        if FFMPEG_BIN.exists():
            FFMPEG_RESOLVED = str(FFMPEG_BIN)
            log_ok("ffmpeg готов к работе.")
        else:
            log_err("ffmpeg не найден после установки. Режимы 1 и 3 недоступны.")
    else:
        log_warn("ffmpeg пропущен. Режимы «Лучшее качество» и «MP3» работать не будут.")


def check_yt_dlp() -> None:
    if _ytdlp_ready():
        try:
            ver = subprocess.run(
                [str(YTDLP_BIN), "--version"],
                capture_output=True, text=True, timeout=10,
            ).stdout.strip()
        except Exception:
            ver = "?"
        log_ok(f"yt-dlp {ver}  (_deps/{YTDLP_BIN.name})")
        return
    log_warn(f"yt-dlp не найден в _deps/.")
    install_yt_dlp()


def run_checks() -> None:
    log_info(f"Папка зависимостей: {DEPS_DIR}")
    log_info(f"Папка загрузок    : {DL_DIR}")
    log_sep()
    check_python()
    check_ffmpeg()
    check_yt_dlp()
    DL_DIR.mkdir(parents=True, exist_ok=True)
    log_sep()


# ══════════════════════════════════════════════════════════════════════════════
#  ВВОД ПОЛЬЗОВАТЕЛЯ
# ══════════════════════════════════════════════════════════════════════════════

def _ask(prompt: str) -> str:
    try:
        return input(prompt).strip()
    except (KeyboardInterrupt, EOFError):
        print()
        raise KeyboardInterrupt


def validate_url(url: str) -> tuple[bool, str]:
    if not url:
        return False, "Ссылка не может быть пустой."
    if not _YT_RE.search(url):
        return False, "Не похоже на YouTube-ссылку. Поддерживаются: обычные, Shorts, Live, плейлисты."
    return True, ""


def ask_url() -> str:
    while True:
        url = _ask(f"\n{C.BOLD}  Ссылка на видео:{C.RESET} ")
        ok, reason = validate_url(url)
        if ok:
            return url
        log_err(reason)
        log_info("Попробуй ещё раз.")


def ask_quality() -> str:
    no_ffmpeg = f"  {C.GRAY}[нужен ffmpeg]{C.RESET}" if not FFMPEG_RESOLVED else ""
    print(f"\n{C.BOLD}  Выбери качество:{C.RESET}")
    print(f"  {C.CYAN}1{C.RESET}  — Лучшее качество (HD / 4K){no_ffmpeg}")
    print(f"  {C.CYAN}2{C.RESET}  — Экономичное (360p)")
    print(f"  {C.CYAN}3{C.RESET}  — Только звук (MP3){no_ffmpeg}")
    while True:
        choice = _ask(f"\n{C.BOLD}  Твой выбор [1/2/3]:{C.RESET} ")
        if choice in ("1", "2", "3"):
            return choice
        log_err("Введи 1, 2 или 3.")


def ask_continue() -> bool:
    while True:
        ans = _ask(
            f"\n{C.BOLD}  Скачать ещё?  {C.RESET}"
            f"{C.CYAN}[д]{C.RESET} / {C.RED}[н]{C.RESET}  "
        ).lower()
        if ans in ("д", "да", "y", "yes", ""):
            return True
        if ans in ("н", "нет", "n", "no"):
            return False
        log_err("Введи 'д' или 'н'.")


# ══════════════════════════════════════════════════════════════════════════════
#  ДВИЖОК ЗАГРУЗКИ
# ══════════════════════════════════════════════════════════════════════════════

def _run(*args: str) -> bool:
    """Запускает yt-dlp бинарник напрямую, возвращает True при успехе."""
    result = subprocess.run([str(YTDLP_BIN), *args])
    return result.returncode == 0


def _ffmpeg_args() -> list[str]:
    return ["--ffmpeg-location", FFMPEG_RESOLVED] if FFMPEG_RESOLVED else []


def _playlist_args(
    url: str,
    items: list[int] | None,
    force_single: bool = False,
) -> list[str]:
    """Собирает доп. аргументы yt-dlp для плейлиста.

    force_single=True — пользователь явно выбрал «только это видео»
    из ссылки, которая содержит &list=. Принудительно добавляем
    --no-playlist даже если URL технически является плейлистом.
    """
    args: list[str] = []
    if not force_single and is_playlist_url(url):
        if items:
            args += ["--playlist-items", _fmt_indices(items)]
        # Не прерываемся на ошибках отдельных видео
        args += ["--ignore-errors"]
    else:
        # Одиночное видео — игнорируем плейлист в ссылке
        args += ["--no-playlist"]
    return args


def _try_formats(
    url: str,
    fmt_chain: list[str],
    extra_args: list[str] | None = None,
    force_single: bool = False,
) -> bool:
    """Перебирает форматы по цепочке до первого успеха."""
    tmpl = OUTPUT_TMPL_PLAYLIST if (is_playlist_url(url) and not force_single) else OUTPUT_TMPL
    extra = extra_args or []
    for i, fmt in enumerate(fmt_chain):
        if i > 0:
            log_warn(f"Пробую запасной вариант {i}: {fmt}")
        success = _run(
            *_ffmpeg_args(),
            "-f", fmt,
            "--merge-output-format", "mp4",
            "-o", tmpl,
            *extra,
            url,
        )
        if success:
            return True
    return False


# ══════════════════════════════════════════════════════════════════════════════
#  ПЛЕЙЛИСТ — ОПРЕДЕЛЕНИЕ, ПОЛУЧЕНИЕ ДАННЫХ, ВЫБОР
# ══════════════════════════════════════════════════════════════════════════════

@dataclass
class PlaylistEntry:
    index: int        # порядковый номер в плейлисте (1-based)
    title: str
    url:   str
    duration: int = 0  # секунды, 0 если неизвестно


@dataclass
class PlaylistInfo:
    title:   str
    entries: list[PlaylistEntry]

    @property
    def count(self) -> int:
        return len(self.entries)


def is_playlist_url(url: str) -> bool:
    """True если URL указывает на плейлист (не просто видео с параметром list=)."""
    # playlist?list= — точно плейлист
    if re.search(r"youtube\.com/playlist\?", url, re.IGNORECASE):
        return True
    # watch?v=...&list= — видео внутри плейлиста, спрашиваем пользователя
    if re.search(r"[?&]list=[\w\-]{10,}", url, re.IGNORECASE):
        return True
    return False


def _fmt_duration(seconds: int) -> str:
    if seconds <= 0:
        return "  ??:??"
    m, s = divmod(seconds, 60)
    h, m = divmod(m, 60)
    if h:
        return f"{h:2d}:{m:02d}:{s:02d}"
    return f"   {m:2d}:{s:02d}"


def fetch_playlist_info(url: str) -> PlaylistInfo | None:
    """
    Получает метаданные плейлиста через yt-dlp бинарник (--flat-playlist --dump-json).
    Не требует Python-модуля yt_dlp вообще.
    Возвращает PlaylistInfo или None при ошибке.
    """
    import json as _json

    log_info("Получаю информацию о плейлисте…")
    try:
        result = subprocess.run(
            [
                str(YTDLP_BIN),
                "--flat-playlist",       # только метаданные, без скачивания
                "--dump-json",           # каждое видео — отдельная JSON-строка в stdout
                "--quiet",
                "--ignore-errors",
                "--no-warnings",
                url,
            ],
            capture_output=True,
            text=True,
            timeout=60,
        )
    except subprocess.TimeoutExpired:
        log_err("Превышено время ожидания при получении плейлиста.")
        return None
    except Exception as exc:
        log_err(f"Не удалось запустить yt-dlp: {exc}")
        return None

    # Каждая строка stdout — JSON одного видео
    lines = [l.strip() for l in result.stdout.splitlines() if l.strip()]
    if not lines:
        log_warn("Плейлист пуст или все видео недоступны.")
        return None

    entries: list[PlaylistEntry] = []
    playlist_title = "playlist"
    for i, line in enumerate(lines, start=1):
        try:
            e = _json.loads(line)
        except _json.JSONDecodeError:
            continue
        # Заголовок плейлиста берём из первого элемента
        if i == 1:
            playlist_title = (
                e.get("playlist_title")
                or e.get("playlist")
                or "playlist"
            )
        entries.append(PlaylistEntry(
            index    = i,
            title    = e.get("title") or e.get("id") or f"Видео {i}",
            url      = e.get("url") or e.get("webpage_url") or url,
            duration = int(e.get("duration") or 0),
        ))

    if not entries:
        log_warn("Не удалось разобрать ни одного видео из плейлиста.")
        return None

    return PlaylistInfo(title=playlist_title, entries=entries)


def _print_playlist(info: PlaylistInfo, start: int = 0, page: int = 25) -> int:
    """
    Выводит страницу из page видео начиная с индекса start.
    Возвращает индекс следующей непоказанной записи.
    """
    end = min(start + page, info.count)
    for e in info.entries[start:end]:
        dur   = _fmt_duration(e.duration)
        title = e.title if len(e.title) <= 55 else e.title[:52] + "…"
        num   = f"{C.CYAN}{e.index:>4}{C.RESET}"
        print(f"  {num}.  {title:<55}  {C.GRAY}{dur}{C.RESET}")
    return end


def _parse_selection(raw: str, max_idx: int) -> list[int] | None:
    """
    Парсит строку выбора вида:  «все» / «1-10» / «1,3,5-8,12»
    Возвращает отсортированный список индексов (1-based) или None при ошибке.
    """
    raw = raw.strip().lower()
    if raw in ("а", "all", "все", "всё", "*"):
        return list(range(1, max_idx + 1))

    result: set[int] = set()
    parts = re.split(r"[,;\s]+", raw)
    for part in parts:
        part = part.strip()
        if not part:
            continue
        m_range = re.fullmatch(r"(\d+)\s*[-–]\s*(\d+)", part)
        m_single = re.fullmatch(r"(\d+)", part)
        if m_range:
            a, b = int(m_range.group(1)), int(m_range.group(2))
            if a > b:
                a, b = b, a
            if a < 1 or b > max_idx:
                log_err(f"Диапазон {a}-{b} выходит за пределы 1–{max_idx}.")
                return None
            result.update(range(a, b + 1))
        elif m_single:
            n = int(m_single.group(1))
            if n < 1 or n > max_idx:
                log_err(f"Номер {n} выходит за пределы 1–{max_idx}.")
                return None
            result.add(n)
        else:
            log_err(f"Непонятный ввод: «{part}». Используй числа, диапазоны (1-5) или «а» для всех.")
            return None

    if not result:
        log_err("Не выбрано ни одного видео.")
        return None
    return sorted(result)


def ask_playlist_mode(url: str) -> tuple[str, list[int] | None] | None:
    """
    Если URL — плейлист, спрашивает пользователя что качать.
    Возвращает (url_для_скачивания, список_номеров_или_None_для_всех)
    или None если пользователь выбрал «скачать как одиночное видео».
    """
    if not is_playlist_url(url):
        return None

    # Если watch?v=...&list= — сначала спросим: плейлист или одно видео?
    is_watch_with_list = bool(re.search(r"youtube\.com/watch\?.*v=[\w\-]{11}.*[?&]list=", url, re.I))
    if is_watch_with_list:
        print(f"\n{C.YELLOW}  !  Ссылка содержит и видео, и плейлист.{C.RESET}")
        print(f"  {C.CYAN}1{C.RESET}  — Скачать только это видео")
        print(f"  {C.CYAN}2{C.RESET}  — Открыть плейлист и выбрать")
        while True:
            ch = _ask(f"\n{C.BOLD}  Твой выбор [1/2]:{C.RESET} ")
            if ch == "1":
                return None   # одиночное
            if ch == "2":
                break
            log_err("Введи 1 или 2.")

    info = fetch_playlist_info(url)
    if not info:
        log_warn("Не удалось загрузить плейлист. Попробую как одиночное видео.")
        return None

    # ── Показываем плейлист ────────────────────────────────────────────────
    print(f"\n{C.BOLD}{C.WHITE}  Плейлист: «{info.title}»{C.RESET}  "
          f"{C.GRAY}({info.count} видео){C.RESET}")
    log_sep()

    shown = _print_playlist(info, start=0, page=25)

    # Пагинация: если видео больше 25 — предлагаем показать ещё
    while shown < info.count:
        remaining = info.count - shown
        ans = _ask(
            f"\n{C.GRAY}  … ещё {remaining} видео. "
            f"Показать? {C.RESET}{C.CYAN}[д]{C.RESET}/{C.RED}[н]{C.RESET}  "
        ).lower()
        if ans in ("д", "да", "y", "yes", ""):
            shown = _print_playlist(info, start=shown, page=25)
        else:
            break

    log_sep()

    # ── Выбор что качать ──────────────────────────────────────────────────
    print(f"\n{C.BOLD}  Что скачать?{C.RESET}")
    print(f"  {C.CYAN}а{C.RESET}   — Все {info.count} видео")
    print(f"  {C.CYAN}1-5{C.RESET} — Диапазон номеров  (например: 1-10)")
    print(f"  {C.CYAN}1,3{C.RESET} — Конкретные номера (например: 1,4,7)")
    print(f"  {C.CYAN}Смесь{C.RESET}: 1-3,7,10-12")

    while True:
        raw = _ask(f"\n{C.BOLD}  Выбор:{C.RESET} ")
        indices = _parse_selection(raw, info.count)
        if indices is not None:
            break

    if len(indices) == info.count:
        log_info(f"Выбраны все {info.count} видео.")
        return url, None      # --playlist-items не нужен → качаем всё
    else:
        log_info(f"Выбрано: {len(indices)} видео  ({_fmt_indices(indices)})")
        return url, indices


def _fmt_indices(indices: list[int]) -> str:
    """Красиво форматирует список номеров: [1,2,3,5,6] → '1-3,5-6'."""
    if not indices:
        return ""
    parts: list[str] = []
    start = end = indices[0]
    for n in indices[1:]:
        if n == end + 1:
            end = n
        else:
            parts.append(f"{start}-{end}" if start != end else str(start))
            start = end = n
    parts.append(f"{start}-{end}" if start != end else str(start))
    return ",".join(parts)


# ══════════════════════════════════════════════════════════════════════════════
#  РЕЖИМЫ СКАЧИВАНИЯ
# ══════════════════════════════════════════════════════════════════════════════

def download_best(
    url: str,
    playlist_items: list[int] | None = None,
    force_single: bool = False,
) -> bool:
    log_info("Скачиваю в лучшем качестве…")
    extra = _playlist_args(url, playlist_items, force_single)
    ok = _try_formats(url, [
        "bestvideo+bestaudio/best",
        "bestvideo+bestaudio",
        "best",
    ], extra_args=extra, force_single=force_single)
    if ok:
        log_ok(f"Готово! → {DL_DIR}")
    else:
        log_err("Не удалось скачать. Попробуй обновить: python VolRenDownloader.py --update")
    return ok


def download_360p(
    url: str,
    playlist_items: list[int] | None = None,
    force_single: bool = False,
) -> bool:
    log_info("Скачиваю в экономичном качестве (360p)…")
    extra = _playlist_args(url, playlist_items, force_single)
    ok = _try_formats(url, [
        "bestvideo[height<=360]+bestaudio/best[height<=360]",
        "best[height<=360]",
        "worst",
    ], extra_args=extra, force_single=force_single)
    if ok:
        log_ok(f"Готово! → {DL_DIR}")
    else:
        log_err("Не удалось скачать. Попробуй обновить: python VolRenDownloader.py --update")
    return ok


def download_mp3(
    url: str,
    playlist_items: list[int] | None = None,
    force_single: bool = False,
) -> bool:
    log_info("Извлекаю аудио в MP3 (лучшее качество VBR)…")
    extra = _playlist_args(url, playlist_items, force_single)
    tmpl  = OUTPUT_TMPL_PLAYLIST if (is_playlist_url(url) and not force_single) else OUTPUT_TMPL
    ok = _run(
        *_ffmpeg_args(),
        "--extract-audio",
        "--audio-format", "mp3",
        "--audio-quality", "0",
        "-o", tmpl,
        *extra,
        url,
    )
    if ok:
        log_ok(f"MP3 готов! → {DL_DIR}")
    else:
        log_err("Не удалось. Убедись что ffmpeg установлен (_deps/).")
    return ok


# ══════════════════════════════════════════════════════════════════════════════
#  ОБНОВЛЕНИЕ ЗАВИСИМОСТЕЙ
# ══════════════════════════════════════════════════════════════════════════════

def update_deps() -> None:
    """Перекачивает бинарники yt-dlp и ffmpeg на последние версии."""
    print(BANNER)
    log_info("Обновляю зависимости…")
    log_sep()

    # Обновить yt-dlp — просто скачать бинарник заново
    log_info("Обновляю yt-dlp…")
    if YTDLP_BIN.exists():
        YTDLP_BIN.unlink()
    install_yt_dlp()

    # Переустановить ffmpeg
    log_info("Переустанавливаю ffmpeg…")
    for f in (DEPS_DIR / "ffmpeg.exe", DEPS_DIR / "ffmpeg",
              DEPS_DIR / "ffprobe.exe", DEPS_DIR / "ffprobe"):
        if f.exists():
            f.unlink()
    install_ffmpeg()

    log_sep()
    log_ok("Обновление завершено.")
    input(f"\n{C.GRAY}  Нажми Enter для выхода…{C.RESET}")


# ══════════════════════════════════════════════════════════════════════════════
#  СЕССИЯ
# ══════════════════════════════════════════════════════════════════════════════

@dataclass
class Session:
    success: int = 0
    failed:  int = 0
    items:   list[str] = field(default_factory=list)

    @property
    def total(self) -> int:
        return self.success + self.failed

    def record(self, label: str, url: str, ok: bool) -> None:
        if ok:
            self.success += 1
        else:
            self.failed += 1
        badge = f"{C.GREEN}OK  {C.RESET}" if ok else f"{C.RED}FAIL{C.RESET}"
        short_url = url if len(url) <= 48 else url[:45] + "…"
        self.items.append(f"  [{badge}]  {label:<16}  {short_url}")

    def print_summary(self) -> None:
        if self.total == 0:
            return
        print(f"\n{C.BOLD}{C.WHITE}  Итоги сессии:{C.RESET}")
        log_sep()
        for item in self.items:
            print(item)
        log_sep()
        print(
            f"  Всего: {C.BOLD}{self.total}{C.RESET}  "
            f"·  {C.GREEN}Успешно: {self.success}{C.RESET}  "
            f"·  {C.RED}Ошибок: {self.failed}{C.RESET}\n"
        )


# ══════════════════════════════════════════════════════════════════════════════
#  ГЛАВНЫЙ ЦИКЛ
# ══════════════════════════════════════════════════════════════════════════════

_LABELS   = {"1": "Лучшее качество", "2": "360p", "3": "MP3"}
_HANDLERS = {"1": download_best,    "2": download_360p, "3": download_mp3}


def download_loop(session: Session) -> None:
    while True:
        url = ask_url()

        # ── Определяем: плейлист или одиночное видео ──────────────────────
        playlist_items: list[int] | None = None
        force_single: bool = False
        playlist_result = ask_playlist_mode(url)

        if playlist_result is not None:
            # playlist_result = (url, indices | None)
            url, playlist_items = playlist_result
            count_str = (
                f"{len(playlist_items)} видео"
                if playlist_items else "весь плейлист"
            )
            log_info(f"Режим плейлиста — {count_str}")
        elif is_playlist_url(url):
            # Принудительно запрещаем скачивание плейлиста
            force_single = True
            log_info("Режим: одиночное видео (плейлист проигнорирован)")

        quality = ask_quality()
        label   = _LABELS[quality]

        log_sep()
        ok = _HANDLERS[quality](url, playlist_items, force_single)

        # В сессию записываем с пометкой о плейлисте
        rec_label = label
        if playlist_result is not None:
            n = len(playlist_items) if playlist_items else "all"
            rec_label = f"{label} [плейлист/{n}]"
        session.record(rec_label, url, ok)
        log_sep()

        if not ask_continue():
            break


def main() -> None:
    # Флаг --update — обновить зависимости и выйти
    if "--update" in sys.argv:
        update_deps()
        return

    print(BANNER)
    run_checks()

    session = Session()
    try:
        download_loop(session)
    except KeyboardInterrupt:
        print(f"\n{C.YELLOW}  Прервано пользователем.{C.RESET}")

    session.print_summary()
    input(f"{C.GRAY}  Нажми Enter для выхода…{C.RESET}")


if __name__ == "__main__":
    main()
