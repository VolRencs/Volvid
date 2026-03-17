"""
VolRen Video/Audio Downloader  —  версия 2.2.1
Автор : VolRen
Инфо  : Все зависимости (ffmpeg, yt-dlp) скачиваются автоматически
        в папку _deps/ рядом со скриптом. Ничего не устанавливается
        в систему. Работает на Windows и Linux (x64 / arm64).
Нужно : Python 3.13+
"""

import json
import os
import platform
import queue
import re
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
import zipfile
from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from pathlib import Path

# ══════════════════════════════════════════════════════════════════════════════
#  КОНФИГ
# ══════════════════════════════════════════════════════════════════════════════

VERSION    = "2.2.1"
SCRIPT_DIR = (
    Path(sys.executable).resolve().parent
    if getattr(sys, "frozen", False)
    else Path(__file__).resolve().parent
)
DEPS_DIR = SCRIPT_DIR / "_deps"
DL_DIR   = SCRIPT_DIR / "downloads"

IS_WINDOWS = sys.platform == "win32"
ARCH       = platform.machine().lower()

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
    os.system("")   # включает VT-режим в cmd / powershell


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
#  ПРОГРЕСС-БАР  (единый — для зависимостей, видео и аудио)
# ══════════════════════════════════════════════════════════════════════════════

_BAR_WIDTH       = 32     # ширина символьного бара (одиночное видео)
_BOARD_BAR_WIDTH = 22     # ширина бара в параллельном дашборде


def _fmt_bytes(n: int) -> str:
    for threshold, label, decimals in (
        (1_073_741_824, "ГБ", 2),
        (1_048_576,     "МБ", 1),
        (1_024,         "КБ", 0),
    ):
        if n >= threshold:
            return f"{n / threshold:.{decimals}f} {label}"
    return f"{n} Б"


def _unit_to_mult(unit: str) -> int:
    key = unit.upper().replace("IB", "").replace("B", "") or ""
    return {
        "":  1,
        "K": 1_024,
        "M": 1_048_576,
        "G": 1_073_741_824,
        "T": 1_099_511_627_776,
    }.get(key, 1)


class ProgressBar:

    def __init__(self, title: str = "") -> None:
        self.title     = title[:60] if title else ""
        self._shown    = False   # заголовок (title) уже напечатан
        self._finished = False   # finish() уже был вызван

    def update(
        self,
        pct:     float,
        done_b:  int,
        total_b: int,
        speed:   str = "",
        eta:     str = "",
    ) -> None:
        """Перерисовывает строку прогресса (\\r, без перевода строки)."""
        with _print_lock:
            # Заголовок — выводится один раз, на отдельной строке
            if not self._shown:
                if self.title:
                    print(f"  {C.WHITE}{C.BOLD}{self.title}{C.RESET}")
                self._shown = True

            filled  = min(int(_BAR_WIDTH * pct / 100), _BAR_WIDTH)
            empty   = _BAR_WIDTH - filled
            bar_str = f"{C.GREEN}{'█' * filled}{C.GRAY}{'░' * empty}{C.RESET}"

            size_str = (
                f"{_fmt_bytes(done_b)}{C.GRAY}/{C.RESET}{_fmt_bytes(total_b)}"
                if total_b > 0
                else (_fmt_bytes(done_b) if done_b > 0 else "…")
            )

            line = (
                f"\r  {C.CYAN}↓{C.RESET}  [{bar_str}]"
                f"  {C.BOLD}{pct:5.1f}%{C.RESET}"
                f"  {C.WHITE}{size_str}{C.RESET}"
            )
            if speed:
                line += f"  {C.GRAY}{speed}{C.RESET}"
            if eta:
                line += f"  {C.GRAY}ETA {eta}{C.RESET}"

            print(line, end="", flush=True)

    def urllib_hook(self, n: int, bs: int, total: int) -> None:
        """Хук для urllib.request.urlretrieve (reporthook=...)."""
        done = min(n * bs, total) if total > 0 else n * bs
        pct  = (done / total * 100) if total > 0 else 0.0
        self.update(pct, done, max(total, 0))

    def finish(self) -> None:
        """Переводит строку после завершения прогресса. Идемпотентен."""
        with _print_lock:
            if self._shown and not self._finished:
                print()
                self._finished = True


# ══════════════════════════════════════════════════════════════════════════════
#  ПАРАЛЛЕЛЬНЫЙ ДАШБОРД  (N строк, cursor-up перерисовка, ~10 fps)
# ══════════════════════════════════════════════════════════════════════════════

@dataclass
class _Slot:
    status:  str   = "idle"
    title:   str   = ""
    pct:     float = 0.0
    done_b:  int   = 0
    total_b: int   = 0
    speed:   str   = ""
    eta:     str   = ""
    label:   str   = ""


class MultiProgressBoard:

    def __init__(self, workers: int, total: int) -> None:
        self._slots       = [_Slot() for _ in range(workers)]
        self._total       = total
        self._done        = 0
        self._failed      = 0
        self._lock        = threading.Lock()
        self._started     = False
        self._last_render = 0.0
        self._lines       = workers * 2 + 2   # 2 строки/слот + разделитель + статус

    def _render_locked(self, force: bool = False) -> None:
        now = time.monotonic()
        if not force and self._started and now - self._last_render < 0.10:
            return
        self._last_render = now
        out: list[str] = []

        for i, s in enumerate(self._slots):
            badge = f"{C.CYAN}[{i + 1}]{C.RESET}"
            match s.status:
                case "idle":
                    out += [f"  {badge}  {C.GRAY}── ожидание ──{C.RESET}", ""]

                case "dl":
                    filled   = min(int(_BOARD_BAR_WIDTH * s.pct / 100), _BOARD_BAR_WIDTH)
                    bar      = f"{C.GREEN}{'█' * filled}{C.GRAY}{'░' * (_BOARD_BAR_WIDTH - filled)}{C.RESET}"
                    size     = (f"{_fmt_bytes(s.done_b)}{C.GRAY}/{C.RESET}{_fmt_bytes(s.total_b)}"
                                if s.total_b else _fmt_bytes(s.done_b) or "…")
                    extra    = (f"  {C.GRAY}{s.speed}{C.RESET}" if s.speed else "")
                    extra   += (f"  {C.GRAY}ETA {s.eta}{C.RESET}" if s.eta else "")
                    out += [
                        f"  {badge}  {C.BOLD}{s.title}{C.RESET}",
                        f"       {C.CYAN}↓{C.RESET}  [{bar}]  {C.BOLD}{s.pct:5.1f}%{C.RESET}"
                        f"  {C.WHITE}{size}{C.RESET}{extra}",
                    ]

                case "merge":
                    out += [
                        f"  {badge}  {C.BOLD}{s.title}{C.RESET}",
                        f"       {C.YELLOW}⚙{C.RESET}  {C.GRAY}{s.label}{C.RESET}",
                    ]

                case "done":
                    full = f"{C.GREEN}{'█' * _BOARD_BAR_WIDTH}{C.RESET}"
                    out += [
                        f"  {badge}  {C.GREEN}✔{C.RESET}  {s.title}",
                        f"       [{full}]  {C.BOLD}100.0%{C.RESET}  {C.GRAY}готово{C.RESET}",
                    ]

                case _:  # fail
                    out += [
                        f"  {badge}  {C.RED}✘{C.RESET}  {s.title}",
                        f"       {C.RED}ошибка загрузки{C.RESET}",
                    ]

        pending = self._total - self._done - self._failed
        out.append(f"  {C.GRAY}{'─' * 54}{C.RESET}")
        out.append(
            f"  {C.GREEN}✔ {self._done}{C.RESET}  {C.RED}✘ {self._failed}{C.RESET}"
            f"  {C.GRAY}◷ {pending} в очереди  │  "
            f"{self._done + self._failed}/{self._total}{C.RESET}"
        )

        if self._started:
            sys.stdout.write(f"\033[{self._lines}A")
        for line in out:
            sys.stdout.write(f"\033[2K\r{line}\n")
        sys.stdout.flush()
        self._started = True

    # ── публичный API ────────────────────────────────────────────────────────

    def start(self, slot: int, title: str) -> None:
        with self._lock:
            self._slots[slot] = _Slot("dl", title[:54])
            self._render_locked(force=True)

    def update(self, slot: int, pct: float, done_b: int, total_b: int,
               speed: str = "", eta: str = "") -> None:
        with self._lock:
            s = self._slots[slot]
            s.pct, s.done_b, s.total_b, s.speed, s.eta = pct, done_b, total_b, speed, eta
            self._render_locked()

    def processing(self, slot: int, label: str) -> None:
        """Переключает слот в режим ffmpeg-обработки с поясняющим лейблом."""
        with self._lock:
            s = self._slots[slot]
            s.status, s.label = "merge", label
            self._render_locked(force=True)

    def finish(self, slot: int, ok: bool) -> None:
        with self._lock:
            s = self._slots[slot]
            s.status = "done" if ok else "fail"
            if ok:
                s.pct = 100.0
                self._done += 1
            else:
                self._failed += 1
            self._render_locked(force=True)

    def reset(self, slot: int) -> None:
        """Сбрасывает слот в ожидание (слот освободился, ждёт следующего видео)."""
        with self._lock:
            self._slots[slot] = _Slot()
            self._render_locked(force=True)

    def finalize(self) -> None:
        with self._lock:
            self._render_locked(force=True)
            sys.stdout.write("\n")
            sys.stdout.flush()


# ══════════════════════════════════════════════════════════════════════════════
#  УСТАНОВКА ЗАВИСИМОСТЕЙ
# ══════════════════════════════════════════════════════════════════════════════

def _download_file(url: str, dest: Path, title: str = "") -> None:
    """Скачивает файл по URL с прогресс-баром."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    bar = ProgressBar(title or dest.name)
    urllib.request.urlretrieve(url, dest, reporthook=bar.urllib_hook)
    bar.finish()


def install_ffmpeg() -> None:
    """Скачивает и распаковывает ffmpeg в _deps/"""
    DEPS_DIR.mkdir(parents=True, exist_ok=True)
    log_info("Скачиваю ffmpeg (Windows)…")
    with tempfile.TemporaryDirectory() as tmp:
        archive = Path(tmp) / "ffmpeg.zip"
        try:
            _download_file(_FFMPEG_WIN_URL, archive, "ffmpeg.zip")
        except Exception as e:
            log_err(f"Ошибка скачивания ffmpeg: {e}")
            sys.exit(1)
        log_info("Распаковываю…")
        targets, found = {"ffmpeg.exe", "ffprobe.exe"}, set()
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
    log_ok(f"ffmpeg установлен в: {DEPS_DIR}")


def _ytdlp_version() -> str | None:
    """Возвращает строку версии yt-dlp или None, если бинарник не готов."""
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
    try:
        _download_file(url, YTDLP_BIN, "yt-dlp")
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
    """Вопрос да/нет: принимает рус. и англ. варианты."""
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
    if raw in ("a", "all", "а", "все", "всё", "*"):
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
    fmt_chain: list[str]


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


def _run_ytdlp(*args: str) -> bool:
    """Запускает yt-dlp бесшумно."""
    return subprocess.run(
        [str(YTDLP_BIN), *args],
        capture_output=True,
    ).returncode == 0

_YTDLP_DL_RE = re.compile(
    r"\[download\]\s+"
    r"(?P<pct>[\d.]+)%\s+of\s+~?\s*"
    r"(?P<size>[\d.]+)\s*(?P<unit>[KMGTkmgt]i?[Bb])",
    re.IGNORECASE,
)
# Скорость и ETA — парсим отдельно, т.к. могут быть "Unknown"
_YTDLP_SPEED_RE = re.compile(
    r"at\s+(?P<speed>[\d.]+\s*[KMGTkmgt]i?[Bb]/s)"
    r"(?:\s+ETA\s+(?P<eta>[\d:]+))?",
    re.IGNORECASE,
)
_YTDLP_DEST_RE = re.compile(r"\[download\]\s+Destination:\s+(.+)")
_YTDLP_PROC_RE = re.compile(r"^\s*\[(Merger|ExtractAudio)\]", re.IGNORECASE)


def _ffmpeg_label(tag: str) -> str:
    """Человекочитаемый лейбл для ffmpeg-операции по тегу из вывода yt-dlp."""
    return "конвертация в MP3 (ffmpeg)…" if "audio" in tag else "слияние видео+аудио (ffmpeg)…"


def _stream_ytdlp(
    args:     list[str],
    on_dest:  "Callable[[str], None] | None" = None,
    on_dl:    "Callable[[float, int, int, str, str], None] | None" = None,
    on_proc:  "Callable[[str], None] | None" = None,
) -> bool:
    """
    Единое ядро: запускает yt-dlp с --newline и читает stdout построчно.
    Колбэки вызываются по типу строки; None-колбэки игнорируются.

      on_dest(filename_stem)
      on_dl(pct, done_b, total_b, speed, eta)
      on_proc(tag)   — tag = 'merger' | 'extractaudio'
    """
    cmd = [str(YTDLP_BIN), "--newline", "--no-warnings", *args]
    try:
        proc = subprocess.Popen(
            cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
            text=True, bufsize=1, encoding="utf-8", errors="replace",
        )
    except Exception as e:
        log_err(f"Не удалось запустить yt-dlp: {e}")
        return False

    for raw in proc.stdout:
        line = raw.rstrip()
        if on_dest and (dm := _YTDLP_DEST_RE.search(line)):
            stem = re.sub(r"^\d+\s*[-–]\s*", "", Path(dm.group(1).strip()).stem)[:58]
            on_dest(stem)
        elif on_dl and (m := _YTDLP_DL_RE.search(line)):
            pct     = float(m.group("pct"))
            total_b = int(float(m.group("size")) * _unit_to_mult(m.group("unit")))
            done_b  = int(total_b * pct / 100)
            speed = eta = ""
            if sm := _YTDLP_SPEED_RE.search(line):
                speed, eta = sm.group("speed") or "", sm.group("eta") or ""
            on_dl(pct, done_b, total_b, speed, eta)
        elif on_proc and (pm := _YTDLP_PROC_RE.search(line)):
            on_proc(pm.group(1).lower())

    proc.wait()
    return proc.returncode == 0


def _run_ytdlp_with_progress(args: list[str], title: str = "") -> bool:
    """Последовательный режим: прогресс-бар + лог ffmpeg-шага."""
    bar        = ProgressBar(title)
    processing = False

    def on_dest(stem: str) -> None:
        nonlocal bar, processing
        bar.finish()
        bar, processing = ProgressBar(stem), False

    def on_dl(pct: float, done_b: int, total_b: int, speed: str, eta: str) -> None:
        nonlocal processing
        processing = False
        bar.update(pct, done_b, total_b, speed=speed, eta=eta)

    def on_proc(tag: str) -> None:
        nonlocal processing
        if not processing:
            bar.finish()
            log_info(_ffmpeg_label(tag).capitalize())
            processing = True

    ok = _stream_ytdlp(args, on_dest=on_dest, on_dl=on_dl, on_proc=on_proc)
    bar.finish()
    return ok


def _run_ytdlp_with_board(args: list[str], board: MultiProgressBoard, slot: int) -> bool:
    """Параллельный режим: прогресс и ffmpeg-статус идут в слот дашборда."""
    return _stream_ytdlp(
        args,
        on_dl   = lambda pct, done_b, total_b, speed, eta:
                      board.update(slot, pct, done_b, total_b, speed=speed, eta=eta),
        on_proc = lambda tag: board.processing(slot, _ffmpeg_label(tag)),
    )


def _build_args(
    cfg:   QualityConfig,
    url:   str,
    tmpl:  str,
    extra: list[str],
    fmt:   str | None = None,
) -> list[str]:
    cmd = [*_ffmpeg_args()]
    if not cfg.fmt_chain:
        cmd += ["--extract-audio", "--audio-format", "mp3", "--audio-quality", "0"]
    else:
        cmd += ["-f", fmt or cfg.fmt_chain[0], "--merge-output-format", "mp4"]
    return [*cmd, "-o", tmpl, *extra, url]


def _run_with_fallback(
    cfg:    QualityConfig,
    url:    str,
    tmpl:   str,
    extra:  list[str],
    runner: "Callable[[list[str]], bool]",
) -> bool:
    """Перебирает fmt_chain; передаёт собранные args в runner-колбэк."""
    if not cfg.fmt_chain:
        return runner(_build_args(cfg, url, tmpl, extra))
    for i, fmt in enumerate(cfg.fmt_chain):
        if i > 0:
            log_warn(f"Запасной формат #{i}: {fmt}")
        if runner(_build_args(cfg, url, tmpl, extra, fmt=fmt)):
            return True
    return False


def _download_entry(
    entry:  PlaylistEntry,
    cfg:    QualityConfig,
    pl_dir: Path,
    board:  MultiProgressBoard,
    slot_q: "queue.SimpleQueue[int]",
) -> tuple[PlaylistEntry, bool]:
    """Один видеофайл для параллельного режима — рисует прогресс в своём слоте."""
    slot = slot_q.get()
    try:
        board.start(slot, entry.title)
        tmpl = str(pl_dir / f"{entry.index:03d} - %(title)s.%(ext)s")
        ok   = _run_with_fallback(
            cfg, entry.url, tmpl, ["--no-playlist"],
            runner=lambda args: _run_ytdlp_with_board(args, board, slot),
        )
        board.finish(slot, ok)
        return entry, ok
    finally:
        time.sleep(0.4)
        board.reset(slot)
        slot_q.put(slot)


def download(
    cfg:          QualityConfig,
    url:          str,
    force_single: bool = False,
    pl_info:      PlaylistInfo | None = None,
    pl_selected:  list[PlaylistEntry] | None = None,
    workers:      int = 1,
) -> bool:
    """Центральная функция загрузки: одиночное видео / плейлист (любой режим)."""
    is_pl = bool(pl_info) and not force_single

    if is_pl and pl_selected:
        pl_dir = DL_DIR / (pl_info.title if pl_info else "playlist")
        pl_dir.mkdir(parents=True, exist_ok=True)
        total  = len(pl_selected)
        failed = 0

        # ── Параллельный плейлист ─────────────────────────────────────────
        if workers > 1:
            log_info(f"Параллельная загрузка: {workers} потока(ов)  ·  {total} видео")
            log_sep()

            board  = MultiProgressBoard(workers, total)
            slot_q: queue.SimpleQueue[int] = queue.SimpleQueue()
            for i in range(workers):
                slot_q.put(i)

            with ThreadPoolExecutor(max_workers=workers) as pool:
                futures  = [pool.submit(_download_entry, e, cfg, pl_dir, board, slot_q)
                            for e in pl_selected]
                results  = [f.result() for f in as_completed(futures)]

            board.finalize()
            log_sep()
            for entry, ok in sorted(results, key=lambda r: r[0].index):
                short = entry.title[:52]
                if ok: log_ok(f"[{entry.index:>3}/{total}]  {short}")
                else:  log_err(f"[{entry.index:>3}/{total}]  FAIL  {short[:47]}")
            failed = sum(1 for _, ok in results if not ok)

        # ── Последовательный плейлист ─────────────────────────────────────
        else:
            log_info(f"Последовательная загрузка: {total} видео")
            log_sep()
            for i, entry in enumerate(pl_selected, 1):
                short = entry.title[:52]
                log_info(f"[{i}/{total}]  {short}")
                tmpl = str(pl_dir / f"{entry.index:03d} - %(title)s.%(ext)s")
                ok   = _run_with_fallback(
                    cfg, entry.url, tmpl, ["--no-playlist"],
                    runner=lambda args, t=short: _run_ytdlp_with_progress(args, t),
                )
                if ok: log_ok(f"[{i}/{total}]  готово")
                else:  failed += 1; log_err(f"[{i}/{total}]  FAIL")

        log_sep()
        log_ok(f"Плейлист завершён  ·  успешно: {total - failed}/{total}")
        return failed == 0

    # ── Одиночное видео ───────────────────────────────────────────────────
    tmpl = str(DL_DIR / "%(title)s.%(ext)s")
    ok   = _run_with_fallback(
        cfg, url, tmpl, ["--no-playlist"],
        runner=lambda args: _run_ytdlp_with_progress(args),
    )
    if ok: log_ok(f"Готово!  →  {DL_DIR}")
    else:  log_err("Не удалось скачать.  Попробуй: python VolRenDownloader.py --update")
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
