"""
VolRen Video/Audio Downloader  —  версия 2.1.0
Автор : VolRen
Инфо  : Все зависимости (ffmpeg, yt-dlp) скачиваются автоматически
        в папку _deps/ рядом со скриптом. Ничего не устанавливается
        в систему. Работает на Windows и Linux (x64 / arm64).
Нужно : Python 3.13+
"""

import json
import os
import platform
import re
import subprocess
import sys
import tempfile
import threading
import urllib.request
import zipfile
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from pathlib import Path

# ══════════════════════════════════════════════════════════════════════════════
#  КОНФИГ
# ══════════════════════════════════════════════════════════════════════════════

VERSION    = "2.1.0"
SCRIPT_DIR = (
    Path(sys.executable).resolve().parent
    if getattr(sys, "frozen", False)
    else Path(__file__).resolve().parent
)
DEPS_DIR = SCRIPT_DIR / "_deps"
DL_DIR   = SCRIPT_DIR / "downloads"

IS_WINDOWS = sys.platform == "win32"
ARCH       = platform.machine().lower()

# Windows: ffmpeg качается в _deps/. Linux: берётся из /usr/bin/
_FFMPEG_WIN_BIN = DEPS_DIR / "ffmpeg.exe"
YTDLP_BIN       = DEPS_DIR / ("yt-dlp.exe" if IS_WINDOWS else "yt-dlp")

_FFMPEG_WIN_URL = (
    "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/"
    "ffmpeg-master-latest-win64-gpl.zip"
)
_YTDLP_BASE = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"


def _ytdlp_url() -> str:
    match (IS_WINDOWS, ARCH):
        case (True, _):
            return _YTDLP_BASE + "yt-dlp.exe"
        case (False, a) if a in ("aarch64", "arm64"):
            return _YTDLP_BASE + "yt-dlp_linux_aarch64"
        case _:
            return _YTDLP_BASE + "yt-dlp_linux"

# ══════════════════════════════════════════════════════════════════════════════
#  ЦВЕТА И ВЫВОД
# ══════════════════════════════════════════════════════════════════════════════

if IS_WINDOWS:
    os.system("")  # включает VT-режим в cmd/powershell


class C:
    RESET  = "\033[0m";  BOLD   = "\033[1m"
    RED    = "\033[91m"; GREEN  = "\033[92m"
    YELLOW = "\033[93m"; CYAN   = "\033[96m"
    GRAY   = "\033[90m"; WHITE  = "\033[97m"


_print_lock = threading.Lock()


def _print(*a, **kw) -> None:
    with _print_lock:
        print(*a, **kw)


def log_ok(m: str)   -> None: _print(f"{C.GREEN}  ✔  {m}{C.RESET}")
def log_err(m: str)  -> None: _print(f"{C.RED}  ✘  {m}{C.RESET}")
def log_info(m: str) -> None: _print(f"{C.CYAN}  →  {m}{C.RESET}")
def log_warn(m: str) -> None: _print(f"{C.YELLOW}  !  {m}{C.RESET}")
def log_sep()        -> None: _print(f"{C.GRAY}{'─' * 56}{C.RESET}")


BANNER = f"""{C.CYAN}
 ╔══════════════════════════════════════════════════════╗
 ║         VolRen  Video / Audio  Downloader            ║
 ║         версия {VERSION}  •  powered by yt-dlp           ║
 ╚══════════════════════════════════════════════════════╝
{C.RESET}"""

# ══════════════════════════════════════════════════════════════════════════════
#  УСТАНОВКА ЗАВИСИМОСТЕЙ
# ══════════════════════════════════════════════════════════════════════════════

def _progress_hook(n: int, bs: int, total: int) -> None:
    done = min(n * bs, total) if total > 0 else n * bs
    if total > 0:
        pct = done / total
        bar = "█" * int(30 * pct) + "░" * (30 - int(30 * pct))
        print(
            f"\r  {C.CYAN}↓{C.RESET}  [{bar}] {pct*100:5.1f}%"
            f"  {done/1e6:.1f}/{total/1e6:.1f} МБ",
            end="", flush=True,
        )
    else:
        print(f"\r  {C.CYAN}↓{C.RESET}  {done/1e6:.1f} МБ…", end="", flush=True)


def _download_file(url: str, dest: Path) -> None:
    dest.parent.mkdir(parents=True, exist_ok=True)
    urllib.request.urlretrieve(url, dest, reporthook=_progress_hook)
    print()


def _extract_ffmpeg(archive: Path) -> None:
    """Извлекает ffmpeg.exe / ffprobe.exe из zip-архива."""
    targets = {"ffmpeg.exe", "ffprobe.exe"}
    found: set[str] = set()
    with zipfile.ZipFile(archive) as zf:
        for member in zf.namelist():
            name = Path(member).name
            if name in targets:
                (DEPS_DIR / name).write_bytes(zf.read(member))
                log_ok(f"Извлечён: {name}")
                found.add(name)
    if not found:
        log_err("ffmpeg.exe не найден в архиве — архив повреждён?")
        sys.exit(1)


def install_ffmpeg() -> None:
    """Скачивает ffmpeg в _deps/"""
    DEPS_DIR.mkdir(parents=True, exist_ok=True)
    log_info("Скачиваю ffmpeg (Windows)…")
    with tempfile.TemporaryDirectory() as tmp:
        archive = Path(tmp) / "ffmpeg.zip"
        try:
            _download_file(_FFMPEG_WIN_URL, archive)
        except Exception as e:
            log_err(f"Ошибка скачивания ffmpeg: {e}")
            sys.exit(1)
        log_info("Распаковываю…")
        _extract_ffmpeg(archive)
    log_ok(f"ffmpeg установлен в: {DEPS_DIR}")


def _ytdlp_version() -> str | None:
    """Возвращает строку версии yt-dlp или None если бинарник не готов."""
    if not YTDLP_BIN.exists():
        return None
    try:
        r = subprocess.run(
            [str(YTDLP_BIN), "--version"],
            capture_output=True, text=True, timeout=10,
        )
        return r.stdout.strip() if r.returncode == 0 else None
    except Exception:
        return None


def install_yt_dlp() -> None:
    DEPS_DIR.mkdir(parents=True, exist_ok=True)
    url = _ytdlp_url()
    log_info(f"Скачиваю yt-dlp ({ARCH}, {'Windows' if IS_WINDOWS else 'Linux'})…")
    log_info(f"Источник: {url}")
    try:
        _download_file(url, YTDLP_BIN)
    except Exception as e:
        log_err(f"Ошибка скачивания yt-dlp: {e}")
        sys.exit(1)
    if not IS_WINDOWS:
        YTDLP_BIN.chmod(YTDLP_BIN.stat().st_mode | 0o111)
    if _ytdlp_version():
        log_ok(f"yt-dlp готов: {YTDLP_BIN.name}")
    else:
        log_err("Бинарник скачан, но не запускается.")
        sys.exit(1)

# ══════════════════════════════════════════════════════════════════════════════
#  ПРОВЕРКИ ОКРУЖЕНИЯ
# ══════════════════════════════════════════════════════════════════════════════

FFMPEG_RESOLVED: str | None = None


def check_python() -> None:
    major, minor = sys.version_info[:2]
    if (major, minor) < (3, 13):
        log_err(f"Python {major}.{minor} не поддерживается. Нужен 3.13+.")
        sys.exit(1)
    log_ok(f"Python {major}.{minor}  ({sys.platform} / {ARCH})")


def _system_ffmpeg() -> str | None:
    """Путь к /usr/bin/ffmpeg если существует, иначе None."""
    p = Path("/usr/bin/ffmpeg")
    return str(p) if p.exists() else None


def check_ffmpeg() -> None:
    global FFMPEG_RESOLVED

    if IS_WINDOWS:
        if _FFMPEG_WIN_BIN.exists():
            FFMPEG_RESOLVED = str(_FFMPEG_WIN_BIN)
            log_ok("ffmpeg найден в _deps/")
            return
        log_warn("ffmpeg не найден в _deps/.")
        if _ask_yes(
            f"  {C.BOLD}Скачать ffmpeg?{C.RESET} {C.CYAN}[д]{C.RESET}/{C.RED}[н]{C.RESET}  "
        ):
            install_ffmpeg()
            if _FFMPEG_WIN_BIN.exists():
                FFMPEG_RESOLVED = str(_FFMPEG_WIN_BIN)
                log_ok("ffmpeg готов.")
            else:
                log_err("ffmpeg не найден после установки. Режимы 1 и 3 недоступны.")
        else:
            log_warn("ffmpeg пропущен. Режимы «Лучшее качество» и «MP3» недоступны.")
        return

    if path := _system_ffmpeg():
        FFMPEG_RESOLVED = path
        log_ok(f"ffmpeg найден в системе: {path}")
    else:
        log_warn("ffmpeg не найден в системе.")
        log_warn("Установи через пакетный менеджер:")
        log_warn("  sudo apt install ffmpeg   (Debian / Ubuntu)")
        log_warn("  sudo dnf install ffmpeg   (Fedora / RHEL)")
        log_warn("  sudo pacman -S ffmpeg     (Arch)")
        log_warn("Режимы «Лучшее качество» и «MP3» недоступны.")


def check_yt_dlp() -> None:
    if ver := _ytdlp_version():
        log_ok(f"yt-dlp {ver}  (_deps/{YTDLP_BIN.name})")
        return
    log_warn("yt-dlp не найден в _deps/.")
    install_yt_dlp()


def run_checks() -> None:
    log_info(f"Зависимости: {DEPS_DIR}")
    log_info(f"Загрузки   : {DL_DIR}")
    log_sep()
    check_python()
    check_ffmpeg()
    check_yt_dlp()
    DL_DIR.mkdir(parents=True, exist_ok=True)
    log_sep()

# ══════════════════════════════════════════════════════════════════════════════
#  ВВОД ПОЛЬЗОВАТЕЛЯ
# ══════════════════════════════════════════════════════════════════════════════

_YT_RE = re.compile(
    r"(youtube\.com/(watch\?.*v=|shorts/|live/|playlist\?list=)|youtu\.be/)[\w\-]{1,}",
    re.IGNORECASE,
)
_PLAYLIST_RE = re.compile(
    r"youtube\.com/playlist\?|[?&]list=[\w\-]{10,}",
    re.IGNORECASE,
)

_YES = frozenset(("д", "да", "y", "yes", ""))
_NO  = frozenset(("н", "нет", "n", "no"))


def _ask(prompt: str) -> str:
    try:
        return input(prompt).strip()
    except (KeyboardInterrupt, EOFError):
        print()
        raise KeyboardInterrupt


def _ask_yes(prompt: str) -> bool:
    """Задаёт вопрос да/нет, принимает русские и английские варианты."""
    while True:
        ans = _ask(prompt).lower()
        if ans in _YES: return True
        if ans in _NO:  return False
        log_err("Введи 'д' или 'н'.")


def ask_url() -> str:
    while True:
        url = _ask(f"\n{C.BOLD}  Ссылка на видео:{C.RESET} ")
        if not url:
            log_err("Ссылка не может быть пустой.")
        elif not _YT_RE.search(url):
            log_err("Не похоже на YouTube-ссылку.")
        else:
            return url


def ask_quality() -> str:
    noff = f"  {C.GRAY}[нужен ffmpeg]{C.RESET}" if not FFMPEG_RESOLVED else ""
    print(f"\n{C.BOLD}  Выбери качество:{C.RESET}")
    print(f"  {C.CYAN}1{C.RESET}  — Лучшее качество (HD / 4K){noff}")
    print(f"  {C.CYAN}2{C.RESET}  — Экономичное (360p)")
    print(f"  {C.CYAN}3{C.RESET}  — Только звук (MP3){noff}")
    while True:
        ch = _ask(f"\n{C.BOLD}  Твой выбор [1/2/3]:{C.RESET} ")
        if ch in ("1", "2", "3"): return ch
        log_err("Введи 1, 2 или 3.")


def ask_continue() -> bool:
    return _ask_yes(
        f"\n{C.BOLD}  Скачать ещё?  {C.RESET}"
        f"{C.CYAN}[д]{C.RESET} / {C.RED}[н]{C.RESET}  "
    )


def ask_workers(total_videos: int) -> int:
    max_w = min(5, total_videos)
    print(f"\n{C.BOLD}  Параллельная загрузка:{C.RESET}  {C.GRAY}(видео: {total_videos}){C.RESET}")
    for i in range(1, max_w + 1):
        desc = "Последовательно" if i == 1 else f"{i} потока(ов)"
        note = f"  {C.GRAY}(рекомендуется){C.RESET}" if i == 3 else ""
        print(f"  {C.CYAN}{i}{C.RESET}  — {desc}{note}")
    while True:
        ch = _ask(f"\n{C.BOLD}  Потоков [1-{max_w}]:{C.RESET} ")
        if ch.isdigit() and 1 <= int(ch) <= max_w:
            return int(ch)
        log_err(f"Введи число от 1 до {max_w}.")

# ══════════════════════════════════════════════════════════════════════════════
#  ПЛЕЙЛИСТ
# ══════════════════════════════════════════════════════════════════════════════

@dataclass(slots=True)
class PlaylistEntry:
    index:    int
    title:    str
    url:      str
    duration: int = 0


@dataclass(slots=True)
class PlaylistInfo:
    title:   str
    entries: list[PlaylistEntry]

    def __len__(self) -> int:
        return len(self.entries)


def is_playlist_url(url: str) -> bool:
    return bool(_PLAYLIST_RE.search(url))


def _fmt_duration(secs: int) -> str:
    if secs <= 0:
        return "  ??:??"
    m, s = divmod(secs, 60)
    h, m = divmod(m, 60)
    return f"{h:2d}:{m:02d}:{s:02d}" if h else f"   {m:2d}:{s:02d}"


def _fmt_indices(idx: list[int]) -> str:
    """[1,2,3,5,6] → '1-3,5-6'"""
    if not idx:
        return ""
    parts: list[str] = []
    start = end = idx[0]
    for n in idx[1:]:
        if n == end + 1:
            end = n
        else:
            parts.append(f"{start}-{end}" if start != end else str(start))
            start = end = n
    parts.append(f"{start}-{end}" if start != end else str(start))
    return ",".join(parts)


def fetch_playlist_info(url: str) -> PlaylistInfo | None:
    log_info("Получаю информацию о плейлисте…")
    try:
        result = subprocess.run(
            [str(YTDLP_BIN), "--flat-playlist", "--dump-json",
             "--quiet", "--ignore-errors", "--no-warnings", url],
            capture_output=True, text=True, timeout=60,
        )
    except subprocess.TimeoutExpired:
        log_err("Таймаут при получении плейлиста.")
        return None
    except Exception as e:
        log_err(f"Ошибка yt-dlp: {e}")
        return None

    lines = [ln.strip() for ln in result.stdout.splitlines() if ln.strip()]
    if not lines:
        log_warn("Плейлист пуст или недоступен.")
        return None

    entries: list[PlaylistEntry] = []
    pl_title = "playlist"
    for i, line in enumerate(lines, 1):
        try:
            e = json.loads(line)
        except json.JSONDecodeError:
            continue
        if i == 1:
            pl_title = e.get("playlist_title") or e.get("playlist") or "playlist"
        entries.append(PlaylistEntry(
            index    = i,
            title    = e.get("title") or e.get("id") or f"Видео {i}",
            url      = e.get("url") or e.get("webpage_url") or url,
            duration = int(e.get("duration") or 0),
        ))

    return PlaylistInfo(title=pl_title, entries=entries) if entries else None


def _print_playlist_page(info: PlaylistInfo, start: int = 0, page: int = 25) -> int:
    end = min(start + page, len(info))
    for e in info.entries[start:end]:
        title = e.title if len(e.title) <= 55 else e.title[:52] + "…"
        print(f"  {C.CYAN}{e.index:>4}{C.RESET}.  {title:<55}  {C.GRAY}{_fmt_duration(e.duration)}{C.RESET}")
    return end


def _parse_selection(raw: str, max_idx: int) -> list[int] | None:
    raw = raw.strip().lower()
    if raw in ("а", "all", "все", "всё", "*"):
        return list(range(1, max_idx + 1))
    result: set[int] = set()
    for part in re.split(r"[,;\s]+", raw):
        if not part:
            continue
        if m := re.fullmatch(r"(\d+)\s*[-–]\s*(\d+)", part):
            a, b = sorted((int(m.group(1)), int(m.group(2))))
            if a < 1 or b > max_idx:
                log_err(f"Диапазон {a}-{b} вне 1–{max_idx}.")
                return None
            result.update(range(a, b + 1))
        elif m := re.fullmatch(r"(\d+)", part):
            n = int(m.group(1))
            if not 1 <= n <= max_idx:
                log_err(f"Номер {n} вне 1–{max_idx}.")
                return None
            result.add(n)
        else:
            log_err(f"Непонятный ввод: «{part}».")
            return None
    return sorted(result) if result else None


def ask_playlist_mode(url: str) -> tuple[str, PlaylistInfo, list[PlaylistEntry]] | None:
    """Возвращает (url, info, выбранные_entries) или None для одиночного видео."""
    if not is_playlist_url(url):
        return None

    if re.search(r"youtube\.com/watch\?.*v=[\w\-]{11}.*[?&]list=", url, re.I):
        print(f"\n{C.YELLOW}  !  Ссылка содержит и видео, и плейлист.{C.RESET}")
        print(f"  {C.CYAN}1{C.RESET}  — Только это видео")
        print(f"  {C.CYAN}2{C.RESET}  — Открыть плейлист")
        while True:
            ch = _ask(f"\n{C.BOLD}  Твой выбор [1/2]:{C.RESET} ")
            if ch == "1": return None
            if ch == "2": break
            log_err("Введи 1 или 2.")

    info = fetch_playlist_info(url)
    if not info:
        log_warn("Не удалось загрузить плейлист. Скачиваю как одиночное видео.")
        return None

    print(f"\n{C.BOLD}{C.WHITE}  Плейлист: «{info.title}»{C.RESET}  "
          f"{C.GRAY}({len(info)} видео){C.RESET}")
    log_sep()

    shown = _print_playlist_page(info)
    while shown < len(info):
        if _ask_yes(
            f"\n{C.GRAY}  … ещё {len(info) - shown}. Показать? {C.RESET}"
            f"{C.CYAN}[д]{C.RESET}/{C.RED}[н]{C.RESET}  "
        ):
            shown = _print_playlist_page(info, start=shown)
        else:
            break
    log_sep()

    print(f"\n{C.BOLD}  Что скачать?{C.RESET}")
    print(f"  {C.CYAN}а{C.RESET}      — Все {len(info)} видео")
    print(f"  {C.CYAN}1-5{C.RESET}    — Диапазон  (например: 1-10)")
    print(f"  {C.CYAN}1,3,7{C.RESET}  — Конкретные номера")
    print(f"  {C.CYAN}Смесь{C.RESET}: 1-3,7,10-12")

    while True:
        indices = _parse_selection(_ask(f"\n{C.BOLD}  Выбор:{C.RESET} "), len(info))
        if indices is not None:
            break

    idx_set  = set(indices)
    selected = [e for e in info.entries if e.index in idx_set]
    n        = len(selected)
    log_info(f"Выбрано: {n} видео" + (f"  ({_fmt_indices(indices)})" if n < len(info) else " (все)"))
    return url, info, selected

# ══════════════════════════════════════════════════════════════════════════════
#  ДВИЖОК ЗАГРУЗКИ
# ══════════════════════════════════════════════════════════════════════════════

@dataclass(slots=True)
class QualityConfig:
    label:     str
    fmt_chain: list[str]   # пустой список → аудио-режим (MP3)


QUALITIES: dict[str, QualityConfig] = {
    "1": QualityConfig("Лучшее качество", [
             "bestvideo+bestaudio/best",
             "bestvideo+bestaudio",
             "best",
         ]),
    "2": QualityConfig("360p", [
             "bestvideo[height<=360]+bestaudio/best[height<=360]",
             "best[height<=360]",
             "worst",
         ]),
    "3": QualityConfig("MP3", []),
}


def _ffmpeg_args() -> list[str]:
    return ["--ffmpeg-location", FFMPEG_RESOLVED] if FFMPEG_RESOLVED else []


def _run_ytdlp(*args: str, silent: bool = False) -> bool:
    return subprocess.run(
        [str(YTDLP_BIN), *args],
        capture_output=silent,
    ).returncode == 0


def _build_args(
    cfg:   QualityConfig,
    url:   str,
    tmpl:  str,
    extra: list[str],
    fmt:   str | None = None,
) -> list[str]:
    cmd = [*_ffmpeg_args()]
    if not cfg.fmt_chain:
        # Аудио-режим: fmt_chain пустой — извлекаем MP3
        cmd += ["--extract-audio", "--audio-format", "mp3", "--audio-quality", "0"]
    else:
        cmd += ["-f", fmt or cfg.fmt_chain[0], "--merge-output-format", "mp4"]
    return [*cmd, "-o", tmpl, *extra, url]


def _run_with_fallback(
    cfg:    QualityConfig,
    url:    str,
    tmpl:   str,
    extra:  list[str],
    silent: bool = False,
) -> bool:
    if not cfg.fmt_chain:
        return _run_ytdlp(*_build_args(cfg, url, tmpl, extra), silent=silent)
    for i, fmt in enumerate(cfg.fmt_chain):
        if i > 0:
            log_warn(f"Запасной формат {i}: {fmt}")
        if _run_ytdlp(*_build_args(cfg, url, tmpl, extra, fmt=fmt), silent=silent):
            return True
    return False


def _download_entry(
    entry:  PlaylistEntry,
    cfg:    QualityConfig,
    pl_dir: Path,
) -> tuple[PlaylistEntry, bool]:
    tmpl = str(pl_dir / f"{entry.index:03d} - %(title)s.%(ext)s")
    ok   = _run_with_fallback(cfg, entry.url, tmpl, ["--no-playlist"], silent=True)
    return entry, ok


def download(
    cfg:          QualityConfig,
    url:          str,
    force_single: bool = False,
    pl_info:      PlaylistInfo | None = None,
    pl_selected:  list[PlaylistEntry] | None = None,
    workers:      int = 1,
) -> bool:
    """
    Одиночное видео / последовательный плейлист → один процесс yt-dlp.
    workers > 1 с pl_selected → параллельные потоки, по видео на поток.
    """
    is_pl = bool(pl_info) and not force_single

    # ── Параллельный режим ──────────────────────────────────────────────────
    if is_pl and pl_selected and workers > 1:
        pl_dir = DL_DIR / (pl_info.title if pl_info else "playlist")
        pl_dir.mkdir(parents=True, exist_ok=True)
        total  = len(pl_selected)
        failed = 0
        log_info(f"Параллельная загрузка: {workers} потока(ов), {total} видео…")
        log_sep()

        with ThreadPoolExecutor(max_workers=workers) as pool:
            futures = [pool.submit(_download_entry, e, cfg, pl_dir) for e in pl_selected]
            for done, future in enumerate(as_completed(futures), 1):
                entry, ok = future.result()
                if ok:
                    log_ok(f"[{done:>3}/{total}]  {entry.title[:55]}")
                else:
                    failed += 1
                    log_err(f"[{done:>3}/{total}]  FAIL: {entry.title[:50]}")

        log_sep()
        log_ok(f"Плейлист завершён — успешно: {total - failed}/{total}")
        return failed == 0

    # ── Обычный / последовательный режим ───────────────────────────────────
    if is_pl:
        tmpl  = str(DL_DIR / "%(playlist_title)s" / "%(playlist_index)s - %(title)s.%(ext)s")
        extra = ["--ignore-errors"]
        if pl_selected and pl_info and len(pl_selected) < len(pl_info):
            extra += ["--playlist-items", _fmt_indices([e.index for e in pl_selected])]
    else:
        tmpl  = str(DL_DIR / "%(title)s.%(ext)s")
        extra = ["--no-playlist"]

    ok = _run_with_fallback(cfg, url, tmpl, extra)
    if ok:
        log_ok(f"Готово! → {DL_DIR}")
    else:
        log_err("Не удалось скачать. Попробуй: python VolRenDownloader.py --update")
    return ok

# ══════════════════════════════════════════════════════════════════════════════
#  ОБНОВЛЕНИЕ ЗАВИСИМОСТЕЙ
# ══════════════════════════════════════════════════════════════════════════════

def update_deps() -> None:
    print(BANNER)
    log_info("Обновляю зависимости…")
    log_sep()

    log_info("Обновляю yt-dlp…")
    if YTDLP_BIN.exists():
        YTDLP_BIN.unlink()
    install_yt_dlp()

    if IS_WINDOWS:
        log_info("Переустанавливаю ffmpeg…")
        for f in (DEPS_DIR / "ffmpeg.exe", DEPS_DIR / "ffprobe.exe"):
            if f.exists():
                f.unlink()
        install_ffmpeg()
    else:
        prefix = f"ffmpeg системный ({_system_ffmpeg()}) —" if _system_ffmpeg() else "ffmpeg не найден —"
        log_warn(f"{prefix} обновляй через пакетный менеджер:")
        log_warn("  sudo apt install ffmpeg   (Debian / Ubuntu)")
        log_warn("  sudo dnf install ffmpeg   (Fedora / RHEL)")
        log_warn("  sudo pacman -S ffmpeg     (Arch)")

    log_sep()
    log_ok("Обновление завершено.")
    input(f"\n{C.GRAY}  Нажми Enter для выхода…{C.RESET}")

# ══════════════════════════════════════════════════════════════════════════════
#  СЕССИЯ
# ══════════════════════════════════════════════════════════════════════════════

@dataclass(slots=True)
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
        short = url if len(url) <= 48 else url[:45] + "…"
        self.items.append(f"  [{badge}]  {label:<24}  {short}")

    def print_summary(self) -> None:
        if not self.total:
            return
        print(f"\n{C.BOLD}{C.WHITE}  Итоги сессии:{C.RESET}")
        log_sep()
        for item in self.items:
            print(item)
        log_sep()
        print(
            f"  Всего: {C.BOLD}{self.total}{C.RESET}  ·  "
            f"{C.GREEN}Успешно: {self.success}{C.RESET}  ·  "
            f"{C.RED}Ошибок: {self.failed}{C.RESET}\n"
        )

# ══════════════════════════════════════════════════════════════════════════════
#  ГЛАВНЫЙ ЦИКЛ
# ══════════════════════════════════════════════════════════════════════════════

def download_loop(session: Session) -> None:
    while True:
        url          = ask_url()
        force_single = False
        pl_info:     PlaylistInfo | None        = None
        pl_selected: list[PlaylistEntry] | None = None
        workers      = 1

        playlist_result = ask_playlist_mode(url)

        if playlist_result is not None:
            url, pl_info, pl_selected = playlist_result
            n = len(pl_selected)
            log_info(f"Режим плейлиста — {n} видео")
            if n >= 2:
                workers = ask_workers(n)
        elif is_playlist_url(url):
            force_single = True
            log_info("Режим: одиночное видео (плейлист проигнорирован)")

        cfg = QUALITIES[ask_quality()]
        log_sep()

        ok = download(
            cfg          = cfg,
            url          = url,
            force_single = force_single,
            pl_info      = pl_info,
            pl_selected  = pl_selected,
            workers      = workers,
        )

        label = cfg.label + (f" [плейлист/{len(pl_selected)}]" if pl_selected else "")
        session.record(label, url, ok)
        log_sep()

        if not ask_continue():
            break


def main() -> None:
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
