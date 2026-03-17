"""
VolRen Video/Audio Downloader  —  версия 2.3.1
Автор : VolRen
Инфо  : Все зависимости (ffmpeg, yt-dlp) скачиваются автоматически
        в папку _deps/. Работает на Windows и Linux (x64 / arm64).
        Требуется Python 3.13+.
"""

import json
import os
import platform
import queue
import re
import shutil
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

VERSION      = "2.3.1"
GITHUB_REPO  = "VolRencs/YouTubeDownloader"
SCRIPT_DIR = (
    Path(sys.executable).resolve().parent
    if getattr(sys, "frozen", False)
    else Path(__file__).resolve().parent
)
DEPS_DIR = SCRIPT_DIR / "_deps"
DL_DIR   = SCRIPT_DIR / "downloads"

IS_WINDOWS = sys.platform == "win32"
ARCH       = platform.machine().lower()

INVALID_FILENAME_RE = re.compile(r'[<>:"/\\|?*]')


def _sanitize_dirname(name: str) -> str:
    name = INVALID_FILENAME_RE.sub("_", name.strip()).rstrip(" .")[:180] if name and name.strip() else ""
    return name or "playlist"


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


if IS_WINDOWS:
    os.system("")

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


_BAR_WIDTH       = 32
_BOARD_BAR_WIDTH = 22


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
        "":  1, "K": 1_024, "M": 1_048_576,
        "G": 1_073_741_824, "T": 1_099_511_627_776,
    }.get(key, 1)


class ProgressBar:
    def __init__(self, title: str = "") -> None:
        self.title     = title[:60]
        self._shown    = False
        self._finished = False

    def update(self, pct: float, done_b: int, total_b: int, speed: str = "") -> None:
        with _print_lock:
            if not self._shown:
                if self.title: print(f"  {C.WHITE}{C.BOLD}{self.title}{C.RESET}")
                self._shown = True
            filled  = min(int(_BAR_WIDTH * pct / 100), _BAR_WIDTH)
            bar_str = f"{C.GREEN}{'█' * filled}{C.GRAY}{'░' * (_BAR_WIDTH - filled)}{C.RESET}"
            size_str = (f"{_fmt_bytes(done_b)}{C.GRAY}/{C.RESET}{_fmt_bytes(total_b)}"
                        if total_b > 0 else (_fmt_bytes(done_b) if done_b > 0 else "…"))
            line = f"\r  {C.CYAN}↓{C.RESET}  [{bar_str}]  {C.BOLD}{pct:5.1f}%{C.RESET}  {C.WHITE}{size_str}{C.RESET}"
            if speed: line += f"  {C.GRAY}{speed}{C.RESET}"
            print(line, end="", flush=True)

    def urllib_hook(self, n: int, bs: int, total: int) -> None:
        done = min(n * bs, total) if total > 0 else n * bs
        self.update((done / total * 100) if total > 0 else 0.0, done, max(total, 0))

    def finish(self) -> None:
        with _print_lock:
            if self._shown and not self._finished:
                print(); self._finished = True


@dataclass(slots=True)
class _Slot:
    status:  str   = "idle"
    title:   str   = ""
    pct:     float = 0.0
    done_b:  int   = 0
    total_b: int   = 0
    speed:   str   = ""
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
        self._lines       = workers * 2 + 2

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
                    filled = min(int(_BOARD_BAR_WIDTH * s.pct / 100), _BOARD_BAR_WIDTH)
                    bar = f"{C.GREEN}{'█' * filled}{C.GRAY}{'░' * (_BOARD_BAR_WIDTH - filled)}{C.RESET}"
                    size = (f"{_fmt_bytes(s.done_b)}{C.GRAY}/{C.RESET}{_fmt_bytes(s.total_b)}"
                            if s.total_b else _fmt_bytes(s.done_b) or "…")
                    extra = f"  {C.GRAY}{s.speed}{C.RESET}" if s.speed else ""
                    out += [
                        f"  {badge}  {C.BOLD}{s.title}{C.RESET}",
                        f"       {C.CYAN}↓{C.RESET}  [{bar}]  {C.BOLD}{s.pct:5.1f}%{C.RESET}  {C.WHITE}{size}{C.RESET}{extra}",
                    ]
                case "merge":
                    out += [f"  {badge}  {C.BOLD}{s.title}{C.RESET}",
                            f"       {C.YELLOW}⚙{C.RESET}  {C.GRAY}{s.label}{C.RESET}"]
                case "done":
                    full = f"{C.GREEN}{'█' * _BOARD_BAR_WIDTH}{C.RESET}"
                    out += [f"  {badge}  {C.GREEN}✔{C.RESET}  {s.title}",
                            f"       [{full}]  {C.BOLD}100.0%{C.RESET}  {C.GRAY}готово{C.RESET}"]
                case _:
                    out += [f"  {badge}  {C.RED}✘{C.RESET}  {s.title}",
                            f"       {C.RED}ошибка загрузки{C.RESET}"]

        pending = self._total - self._done - self._failed
        out += [
            f"  {C.GRAY}{'─' * 54}{C.RESET}",
            f"  {C.GREEN}✔ {self._done}{C.RESET}  {C.RED}✘ {self._failed}{C.RESET}"
            f"  {C.GRAY}◷ {pending} в очереди  │  {self._done + self._failed}/{self._total}{C.RESET}",
        ]
        if self._started: sys.stdout.write(f"\033[{self._lines}A")
        sys.stdout.write("".join(f"\033[2K\r{l}\n" for l in out))
        sys.stdout.flush()
        self._started = True

    def start(self, slot: int, title: str) -> None:
        with self._lock: self._slots[slot] = _Slot("dl", title[:54]); self._render_locked(force=True)

    def update(self, slot: int, pct: float, done_b: int, total_b: int, speed: str = "") -> None:
        with self._lock:
            s = self._slots[slot]
            s.pct, s.done_b, s.total_b, s.speed = pct, done_b, total_b, speed
            self._render_locked()

    def processing(self, slot: int, label: str) -> None:
        with self._lock:
            s = self._slots[slot]; s.status, s.label = "merge", label; self._render_locked(force=True)

    def finish(self, slot: int, ok: bool) -> None:
        with self._lock:
            s = self._slots[slot]
            s.status = "done" if ok else "fail"
            s.pct    = 100.0 if ok else s.pct
            self._done += ok; self._failed += not ok
            self._render_locked(force=True)

    def reset(self, slot: int) -> None:
        with self._lock: self._slots[slot] = _Slot(); self._render_locked(force=True)

    def finalize(self) -> None:
        with self._lock: self._render_locked(force=True); sys.stdout.write("\n"); sys.stdout.flush()


def _download_file(url: str, dest: Path, title: str = "") -> None:
    dest.parent.mkdir(parents=True, exist_ok=True)
    bar = ProgressBar(title or dest.name)
    urllib.request.urlretrieve(url, dest, reporthook=bar.urllib_hook)
    bar.finish()


def install_ffmpeg() -> None:
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
                if (name := Path(member).name) in targets:
                    (DEPS_DIR / name).write_bytes(zf.read(member))
                    log_ok(f"Извлечён: {name}"); found.add(name)
        if "ffmpeg.exe" not in found:
            log_err("ffmpeg.exe не найден в архиве."); sys.exit(1)
    log_ok(f"ffmpeg установлен в: {DEPS_DIR}")


def _ytdlp_version() -> str | None:
    if not YTDLP_BIN.exists(): return None
    try:
        r = subprocess.run([str(YTDLP_BIN), "--version"], capture_output=True, text=True, timeout=10)
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
    ver = _ytdlp_version()
    (log_ok if ver else log_err)(f"yt-dlp {'готов: ' + YTDLP_BIN.name if ver else 'скачан, но не запускается.'}")
    if not ver:
        sys.exit(1)


FFMPEG_RESOLVED: str | None = None


def check_python() -> None:
    v = sys.version_info[:2]
    if v < (3, 13): log_err(f"Python {v[0]}.{v[1]} не поддерживается. Нужен 3.13+."); sys.exit(1)
    log_ok(f"Python {v[0]}.{v[1]}  ({sys.platform} / {ARCH})")


def check_ffmpeg() -> None:
    global FFMPEG_RESOLVED

    if IS_WINDOWS:
        if _FFMPEG_WIN_BIN.exists():
            FFMPEG_RESOLVED = str(_FFMPEG_WIN_BIN); log_ok("ffmpeg найден в _deps/"); return
        log_warn("ffmpeg не найден в _deps/.")
        if _ask_yes(f"  {C.BOLD}Скачать ffmpeg?{C.RESET} {C.CYAN}[д]{C.RESET}/{C.RED}[н]{C.RESET}  "):
            install_ffmpeg()
            if _FFMPEG_WIN_BIN.exists():
                FFMPEG_RESOLVED = str(_FFMPEG_WIN_BIN); log_ok("ffmpeg готов.")
            else:
                log_err("ffmpeg не найден после установки.")
        else:
            log_warn("ffmpeg пропущен. Режимы 1 и 3 недоступны.")
        return

    if path := shutil.which("ffmpeg"):
        FFMPEG_RESOLVED = path
        log_ok(f"ffmpeg найден в системе: {path}")
    else:
        log_warn("ffmpeg не найден в PATH.")
        for hint in ("Установи через пакетный менеджер:",
                     "  sudo apt install ffmpeg     (Debian / Ubuntu)",
                     "  sudo dnf install ffmpeg     (Fedora / RHEL)",
                     "  sudo pacman -S ffmpeg       (Arch)",
                     "Режимы «Лучшее качество» и «MP3» недоступны."):
            log_warn(hint)


def check_yt_dlp() -> None:
    if ver := _ytdlp_version(): log_ok(f"yt-dlp {ver}  (_deps/{YTDLP_BIN.name})"); return
    log_warn("yt-dlp не найден в _deps/."); install_yt_dlp()


def run_checks() -> None:
    log_info(f"Зависимости: {DEPS_DIR}"); log_info(f"Загрузки   : {DL_DIR}"); log_sep()
    check_python(); check_ffmpeg(); check_yt_dlp()
    DL_DIR.mkdir(parents=True, exist_ok=True); log_sep()


_YT_RE = re.compile(
    r"(youtube\.com/(watch\?.*v=|shorts/|live/|playlist\?list=)|youtu\.be/)[\w\-]+",
    re.IGNORECASE,
)
_PLAYLIST_RE = re.compile(r"youtube\.com/playlist\?|[?&]list=[\w\-]{10,}", re.IGNORECASE)

_YES = frozenset(("д", "да", "y", "yes", ""))
_NO  = frozenset(("н", "нет", "n", "no"))


def _ask(prompt: str) -> str:
    try: return input(prompt).strip()
    except (KeyboardInterrupt, EOFError): print(); raise KeyboardInterrupt


def _ask_yes(prompt: str) -> bool:
    while True:
        ans = _ask(prompt).lower()
        if ans in _YES: return True
        if ans in _NO:  return False
        log_err("Введи 'д' или 'н'.")


def ask_url() -> str:
    while True:
        url = _ask(f"\n{C.BOLD}  Ссылка на видео:{C.RESET} ")
        if   not url:               log_err("Ссылка не может быть пустой.")
        elif not _YT_RE.search(url): log_err("Не похоже на YouTube-ссылку.")
        else: return url


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
    return _ask_yes(f"\n{C.BOLD}  Скачать ещё?  {C.RESET}{C.CYAN}[д]{C.RESET} / {C.RED}[н]{C.RESET}  ")


def ask_workers(total_videos: int) -> int:
    max_w = min(5, total_videos)
    print(f"\n{C.BOLD}  Параллельная загрузка:{C.RESET}  {C.GRAY}(видео: {total_videos}){C.RESET}")
    for i in range(1, max_w + 1):
        desc = "Последовательно" if i == 1 else f"{i} потока(ов)"
        note = f"  {C.GRAY}(рекомендуется){C.RESET}" if i == 3 else ""
        print(f"  {C.CYAN}{i}{C.RESET}  — {desc}{note}")
    while True:
        ch = _ask(f"\n{C.BOLD}  Потоков [1-{max_w}]:{C.RESET} ")
        if ch.isdigit() and 1 <= (n := int(ch)) <= max_w: return n
        log_err(f"Введи число от 1 до {max_w}.")


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
    if secs <= 0: return "  ??:??"
    h, r = divmod(secs, 3600)
    m, s = divmod(r, 60)
    return f"{h:2d}:{m:02d}:{s:02d}" if h else f"   {m:2d}:{s:02d}"


def _fmt_indices(idx: list[int]) -> str:
    if not idx: return ""
    parts, start, end = [], idx[0], idx[0]
    for n in idx[1:]:
        if n == end + 1: end = n
        else: parts.append(f"{start}-{end}" if start != end else str(start)); start = end = n
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
    except Exception as e:
        log_err(f"Ошибка yt-dlp: {e}")
        return None

    lines = [ln.strip() for ln in result.stdout.splitlines() if ln.strip()]
    if not lines:
        log_warn("Плейлист пуст или недоступен.")
        return None

    entries: list[PlaylistEntry] = []
    first: dict = {}
    for i, line in enumerate(lines, 1):
        try:
            e = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not first:
            first = e
        video_url = (
            e.get("url") or e.get("webpage_url")
            or (f"https://youtu.be/{e['id']}" if e.get("id") else None)
        )
        if not video_url:
            log_warn(f"Пропущено видео #{i}: нет URL.")
            continue
        entries.append(PlaylistEntry(i,
            e.get("title") or e.get("id") or f"Видео {i}",
            video_url, int(e.get("duration") or 0),
        ))

    pl_title = first.get("playlist_title") or first.get("playlist") or "playlist"
    return PlaylistInfo(title=pl_title, entries=entries) if entries else None


def _print_playlist_page(info: PlaylistInfo, start: int = 0, page: int = 25) -> int:
    end = min(start + page, len(info))
    for e in info.entries[start:end]:
        title = e.title[:52] + "…" if len(e.title) > 55 else e.title
        print(f"  {C.CYAN}{e.index:>4}{C.RESET}.  {title:<55}  {C.GRAY}{_fmt_duration(e.duration)}{C.RESET}")
    return end


def _parse_selection(raw: str, max_idx: int) -> list[int] | None:
    raw = raw.strip().lower()
    if not raw:
        log_err("Выбор не может быть пустым.")
        return None
    if raw in ("а", "a", "all", "все", "всё", "*"):
        return list(range(1, max_idx + 1))
    result: set[int] = set()
    for part in re.split(r"[,;\s]+", raw):
        if not part: continue
        if m := re.fullmatch(r"(\d+)\s*[-–]\s*(\d+)", part):
            a, b = sorted((int(m.group(1)), int(m.group(2))))
            if a < 1 or b > max_idx: log_err(f"Диапазон {a}-{b} вне 1–{max_idx}."); return None
            result.update(range(a, b + 1))
        elif m := re.fullmatch(r"(\d+)", part):
            n = int(m.group(1))
            if not 1 <= n <= max_idx: log_err(f"Номер {n} вне 1–{max_idx}."); return None
            result.add(n)
        else:
            log_err(f"Непонятный ввод: «{part}»."); return None
    return sorted(result) if result else None


def ask_playlist_mode(url: str) -> tuple[str, PlaylistInfo, list[PlaylistEntry]] | None:
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
    while shown < len(info) and _ask_yes(
        f"\n{C.GRAY}  … ещё {len(info) - shown}. Показать? {C.RESET}{C.CYAN}[д]{C.RESET}/{C.RED}[н]{C.RESET}  "
    ):
        shown = _print_playlist_page(info, start=shown)
    log_sep()

    print(f"\n{C.BOLD}  Что скачать?{C.RESET}")
    print(f"  {C.CYAN}а{C.RESET}      — Все {len(info)} видео")
    print(f"  {C.CYAN}1-5{C.RESET}    — Диапазон")
    print(f"  {C.CYAN}1,3,7{C.RESET}  — Конкретные номера")

    while not (indices := _parse_selection(_ask(f"\n{C.BOLD}  Выбор:{C.RESET} "), len(info))):
        pass

    selected = [e for e in info.entries if e.index in set(indices)]
    log_info(f"Выбрано: {len(selected)} видео" + (f"  ({_fmt_indices(indices)})" if len(selected) < len(info) else " (все)"))
    return url, info, selected


@dataclass(slots=True)
class QualityConfig:
    label:     str
    fmt_chain: list[str]


QUALITIES: dict[str, QualityConfig] = {
    "1": QualityConfig("Лучшее качество", ["bestvideo+bestaudio/best", "bestvideo+bestaudio", "best"]),
    "2": QualityConfig("360p", ["bestvideo[height<=360]+bestaudio/best[height<=360]", "best[height<=360]", "worst"]),
    "3": QualityConfig("MP3", []),
}


def _ffmpeg_args() -> list[str]:
    return ["--ffmpeg-location", FFMPEG_RESOLVED] if FFMPEG_RESOLVED else []


_YTDLP_DL_RE = re.compile(
    r"\[download\]\s+"
    r"(?P<pct>[\d.]+)%\s+of\s+~?\s*"
    r"(?P<size>[\d.]+)\s*(?P<unit>[KMGTkmgt]i?[Bb])",
    re.IGNORECASE,
)
_YTDLP_SPEED_RE = re.compile(
    r"at\s+(?P<speed>[\d.]+\s*[KMGTkmgt]i?[Bb]/s)",
    re.IGNORECASE,
)
_YTDLP_DEST_RE = re.compile(r"\[download\]\s+Destination:\s+(.+)")
_YTDLP_PROC_RE = re.compile(r"^\s*\[(Merger|ExtractAudio)\]", re.IGNORECASE)


def _ffmpeg_label(tag: str) -> str:
    return "конвертация в MP3 (ffmpeg)…" if "audio" in tag else "слияние видео+аудио (ffmpeg)…"


def _stream_ytdlp(
    args:     list[str],
    on_dest:  Callable[[str], None] | None = None,
    on_dl:    Callable[[float, int, int, str], None] | None = None,
    on_proc:  Callable[[str], None] | None = None,
) -> bool:
    cmd = [str(YTDLP_BIN), "--newline", "--no-warnings", *args]
    try:
        proc = subprocess.Popen(
            cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
            text=True, bufsize=1, encoding="utf-8", errors="replace",
        )
    except Exception as e:
        log_err(f"Не удалось запустить yt-dlp: {e}")
        return False

    try:
        for raw in proc.stdout:
            line = raw.rstrip()
            if on_dest and (dm := _YTDLP_DEST_RE.search(line)):
                on_dest(re.sub(r"^\d+\s*[-–]\s*", "", Path(dm.group(1).strip()).stem)[:58])
            elif on_dl and (m := _YTDLP_DL_RE.search(line)):
                pct     = float(m.group("pct"))
                total_b = int(float(m.group("size")) * _unit_to_mult(m.group("unit")))
                sm      = _YTDLP_SPEED_RE.search(line)
                on_dl(pct, int(total_b * pct / 100), total_b,
                      sm.group("speed") if sm else "")
            elif on_proc and (pm := _YTDLP_PROC_RE.search(line)):
                on_proc(pm.group(1).lower())
    finally:
        proc.wait()
    return proc.returncode == 0


def _run_ytdlp_with_progress(args: list[str], title: str = "") -> bool:
    bar = ProgressBar(title)
    processing = False

    def on_dest(stem: str) -> None:
        nonlocal bar, processing
        bar.finish(); bar, processing = ProgressBar(stem), False

    def on_dl(pct: float, done_b: int, total_b: int, speed: str) -> None:
        nonlocal processing
        processing = False; bar.update(pct, done_b, total_b, speed=speed)

    def on_proc(tag: str) -> None:
        nonlocal processing
        if not processing: bar.finish(); log_info(_ffmpeg_label(tag).capitalize()); processing = True

    ok = _stream_ytdlp(args, on_dest=on_dest, on_dl=on_dl, on_proc=on_proc)
    bar.finish(); return ok


def _run_ytdlp_with_board(args: list[str], board: MultiProgressBoard, slot: int) -> bool:
    return _stream_ytdlp(
        args,
        on_dl   = lambda pct, done_b, total_b, speed: board.update(slot, pct, done_b, total_b, speed),
        on_proc = lambda tag: board.processing(slot, _ffmpeg_label(tag)),
    )


def _build_args(cfg: QualityConfig, url: str, tmpl: str, extra: list[str], fmt: str | None = None) -> list[str]:
    cmd = [*_ffmpeg_args()]
    if not cfg.fmt_chain:
        cmd += ["--extract-audio", "--audio-format", "mp3", "--audio-quality", "0"]
    else:
        cmd += ["-f", fmt or cfg.fmt_chain[0], "--merge-output-format", "mp4"]
    return [*cmd, "-o", tmpl, "--windows-filenames", *extra, url]


def _run_with_fallback(cfg: QualityConfig, url: str, tmpl: str, extra: list[str], runner: Callable[[list[str]], bool]) -> bool:
    if not cfg.fmt_chain:
        return runner(_build_args(cfg, url, tmpl, extra))
    for i, fmt in enumerate(cfg.fmt_chain):
        if i > 0: log_warn(f"Запасной формат #{i}: {fmt}")
        if runner(_build_args(cfg, url, tmpl, extra, fmt=fmt)): return True
    return False


def _download_entry(entry: PlaylistEntry, cfg: QualityConfig, pl_dir: Path,
                    board: MultiProgressBoard, slot_q: queue.SimpleQueue[int]) -> tuple[PlaylistEntry, bool]:
    slot = slot_q.get()
    try:
        board.start(slot, entry.title)
        tmpl = str(pl_dir / f"{entry.index:03d} - %(title)s.%(ext)s")
        ok   = _run_with_fallback(cfg, entry.url, tmpl, ["--no-playlist"],
                                  runner=lambda args: _run_ytdlp_with_board(args, board, slot))
        board.finish(slot, ok)
        return entry, ok
    finally:
        time.sleep(0.3); board.reset(slot); slot_q.put(slot)


def download(
    cfg:          QualityConfig,
    url:          str,
    force_single: bool = False,
    pl_info:      PlaylistInfo | None = None,
    pl_selected:  list[PlaylistEntry] | None = None,
    workers:      int = 1,
) -> bool:
    if pl_info and not force_single and pl_selected:
        safe_name = _sanitize_dirname(pl_info.title)
        pl_dir = DL_DIR / safe_name

        try:
            pl_dir.mkdir(parents=True, exist_ok=True)
        except OSError as e:
            log_err(f"Не удалось создать папку «{safe_name}»: {e}")
            pl_dir = DL_DIR / "playlist"
            pl_dir.mkdir(parents=True, exist_ok=True)

        total  = len(pl_selected)
        failed = 0

        if workers > 1:
            log_info(f"Параллельная загрузка: {workers} потоков · {total} видео")
            log_sep()

            board  = MultiProgressBoard(workers, total)
            slot_q: queue.SimpleQueue[int] = queue.SimpleQueue()
            for i in range(workers): slot_q.put(i)

            with ThreadPoolExecutor(max_workers=workers) as pool:
                futures = [pool.submit(_download_entry, e, cfg, pl_dir, board, slot_q)
                           for e in pl_selected]
                results = [f.result() for f in as_completed(futures)]

            board.finalize()
            log_sep()

            for pos, (entry, ok) in enumerate(sorted(results, key=lambda r: r[0].index), 1):
                short = entry.title[:52]
                if ok: log_ok(f"[{pos:>3}/{total}]  {short}")
                else:  failed += 1; log_err(f"[{pos:>3}/{total}]  FAIL  {short[:47]}")

        else:
            log_info(f"Последовательная загрузка: {total} видео")
            log_sep()
            for i, entry in enumerate(pl_selected, 1):
                short = entry.title[:52]
                log_info(f"[{i}/{total}]  {short}")
                tmpl = str(pl_dir / f"{entry.index:03d} - %(title)s.%(ext)s")
                ok = _run_with_fallback(
                    cfg, entry.url, tmpl, ["--no-playlist"],
                    runner=lambda args, t=short: _run_ytdlp_with_progress(args, t),
                )
                if ok: log_ok(f"[{i}/{total}]  готово")
                else:  failed += 1; log_err(f"[{i}/{total}]  FAIL")

        log_sep()
        log_ok(f"Плейлист завершён · успешно: {total - failed}/{total}")
        return failed == 0

    ok = _run_with_fallback(cfg, url, str(DL_DIR / "%(title)s.%(ext)s"), ["--no-playlist"],
                            runner=lambda args: _run_ytdlp_with_progress(args))
    (log_ok if ok else log_err)(f"Готово! → {DL_DIR}" if ok else
                                "Не удалось скачать. Попробуй: python VolRenDownloader.py --update")
    return ok


def update_deps() -> None:
    print(BANNER)
    log_info("Обновляю зависимости…")
    log_sep()

    log_info("Обновляю yt-dlp…")
    if YTDLP_BIN.exists(): YTDLP_BIN.unlink()
    install_yt_dlp()

    if IS_WINDOWS:
        log_info("Переустанавливаю ffmpeg…")
        for f in (DEPS_DIR / "ffmpeg.exe", DEPS_DIR / "ffprobe.exe"):
            if f.exists(): f.unlink()
        install_ffmpeg()
    else:
        path   = shutil.which("ffmpeg")
        prefix = f"ffmpeg системный ({path}) —" if path else "ffmpeg не найден —"
        for hint in (f"{prefix} обновляй через пакетный менеджер:",
                     "  sudo apt install ffmpeg     (Debian / Ubuntu)",
                     "  sudo dnf install ffmpeg     (Fedora / RHEL)",
                     "  sudo pacman -S ffmpeg       (Arch)"):
            log_warn(hint)

    log_sep()
    log_ok("Обновление завершено.")
    input(f"\n{C.GRAY}  Нажми Enter для выхода…{C.RESET}")


@dataclass(slots=True)
class Session:
    success: int = 0
    failed:  int = 0
    items:   list[str] = field(default_factory=list)

    @property
    def total(self) -> int:
        return self.success + self.failed

    def record(self, label: str, url: str, ok: bool) -> None:
        self.success += ok; self.failed += not ok
        badge = f"{C.GREEN}OK  {C.RESET}" if ok else f"{C.RED}FAIL{C.RESET}"
        short = url if len(url) <= 48 else url[:45] + "…"
        self.items.append(f"  [{badge}]  {label:<24}  {short}")

    def print_summary(self) -> None:
        if not self.total: return
        print(f"\n{C.BOLD}{C.WHITE}  Итоги сессии:{C.RESET}"); log_sep()
        print("\n".join(self.items)); log_sep()
        print(f"  Всего: {C.BOLD}{self.total}{C.RESET}  ·  "
              f"{C.GREEN}Успешно: {self.success}{C.RESET}  ·  "
              f"{C.RED}Ошибок: {self.failed}{C.RESET}\n")


def download_loop(session: Session) -> None:
    while True:
        url          = ask_url()
        force_single = False
        pl_info:     PlaylistInfo | None        = None
        pl_selected: list[PlaylistEntry] | None = None
        workers      = 1
        if res := ask_playlist_mode(url):
            url, pl_info, pl_selected = res
            n = len(pl_selected)
            log_info(f"Режим плейлиста — {n} видео")
            if n >= 2: workers = ask_workers(n)
        elif is_playlist_url(url):
            force_single = True
            log_info("Режим: одиночное видео (плейлист проигнорирован)")
        cfg = QUALITIES[ask_quality()]
        log_sep()
        ok = download(cfg=cfg, url=url, force_single=force_single,
                      pl_info=pl_info, pl_selected=pl_selected, workers=workers)
        session.record(cfg.label + (f" [плейлист/{len(pl_selected)}]" if pl_selected else ""), url, ok)
        log_sep()
        if not ask_continue(): break

def _check_update() -> None:
    try:
        with urllib.request.urlopen(urllib.request.Request(
            f"https://api.github.com/repos/{GITHUB_REPO}/releases/latest",
            headers={"User-Agent": "VolRenDownloader"},
        ), timeout=8) as r:
            data = json.loads(r.read())
    except Exception:
        return
 
    latest = data.get("tag_name", "").lstrip("v")
    if not latest or latest <= VERSION or not getattr(sys, "frozen", False): return
 
    log_info(f"Доступна новая версия: {C.BOLD}{latest}{C.RESET}{C.CYAN}  (текущая: {VERSION})")
    if not _ask_yes(f"  {C.BOLD}Обновить сейчас?{C.RESET} {C.CYAN}[д]{C.RESET}/{C.RED}[н]{C.RESET}  "): return
 
    dl_url = next((a["browser_download_url"] for a in data.get("assets", [])
                   if a["name"].endswith(".exe")), None)
    if not dl_url: log_warn("Файл .exe не найден в релизе."); return
 
    dest = Path(sys.executable).resolve()
    tmp  = dest.with_suffix(".new.exe")
    try:
        _download_file(dl_url, tmp, f"VolRenDownloader {latest}")
    except Exception as e:
        log_err(f"Ошибка загрузки: {e}"); tmp.unlink(missing_ok=True); return
 
    bat = dest.with_suffix(".update.bat")
    bat.write_text(
        f"@echo off\ntimeout /t 2 /nobreak >nul\n:retry\n"
        f"move /y \"{tmp}\" \"{dest}\" >nul 2>&1\n"
        f"if errorlevel 1 ( timeout /t 2 /nobreak >nul & goto retry )\n"
        f"del \"%~f0\"\n", encoding="cp866",
    )
    subprocess.Popen(["cmd", "/c", str(bat)],
                     creationflags=subprocess.DETACHED_PROCESS | subprocess.CREATE_NEW_PROCESS_GROUP)
    log_ok(f"Обновление до {latest} установлено. Запустите программу вручную.")
    sys.exit(0)

def main() -> None:
    if "--update" in sys.argv: update_deps(); return
    if getattr(sys, "frozen", False):
        Path(sys.executable).with_suffix(".old.exe").unlink(missing_ok=True)
    if "--no-autoupdate" not in sys.argv: _check_update()
    print(BANNER); run_checks()
    session = Session()
    try:
        download_loop(session)
    except KeyboardInterrupt:
        print(f"\n{C.YELLOW}  Прервано пользователем.{C.RESET}")
    session.print_summary()
    input(f"{C.GRAY}  Нажми Enter для выхода…{C.RESET}")


if __name__ == "__main__":
    main()