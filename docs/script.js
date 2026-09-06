// Typing animation for URL (inside Target box, replaces placeholder)
const url = 'https://youtu.be/dQw4w9WgXcQ';
const typedEl = document.getElementById('typed');
const placeholderEl = typedEl ? typedEl.previousElementSibling : null;
let i = 0, dir = 1;
function typeLoop() {
  if (!typedEl) return;
  if (placeholderEl) placeholderEl.style.display = i > 0 ? 'none' : '';
  typedEl.textContent = url.slice(0, i);
  if (dir === 1) { i++; if (i > url.length) { dir = -1; return setTimeout(typeLoop, 2500); } }
  else { i--; if (i < 0) { dir = 1; i = 0; } }
  setTimeout(typeLoop, dir === 1 ? 60 : 20);
}
typeLoop();
//
// TUI menu keyboard nav (hero menu removed — guard)
const items = [...document.querySelectorAll('#tuiMenu li')];
let sel = 0;
document.addEventListener('keydown', e => {
  if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
  if (!items.length) return;
  e.preventDefault();
  sel = (sel + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length;
  items.forEach((li, k) => { li.classList.toggle('sel', k === sel); li.textContent = (k === sel ? '➤ ' : '') + li.textContent.replace(/^➤\s*/, ''); });
});

// Quality list interactive (numbered picker)
const q = [...document.querySelectorAll('#qList li')];
let qs = 0;
function paintQ() {
  q.forEach((li, k) => li.classList.toggle('sel', k === qs));
}
paintQ();
setInterval(() => { if (q.length) { qs = (qs + 1) % q.length; paintQ(); } }, 1800);
document.addEventListener('keydown', e => {
  if (!q.length) return;
  if (e.key === 'ArrowDown') { qs = (qs + 1) % q.length; paintQ(); }
  if (e.key === 'ArrowUp') { qs = (qs - 1 + q.length) % q.length; paintQ(); }
  const n = parseInt(e.key, 10);
  if (n >= 1 && n <= q.length) { qs = n - 1; paintQ(); }
});

// Live version + release links from GitHub (fallback stays if API is unreachable)
const REPO = 'VolRencs/Volvid';
async function loadVersion() {
  const els = document.querySelectorAll('[data-volvid-version]');
  try {
    const res = await fetch('https://api.github.com/repos/' + REPO + '/releases/latest', {
      headers: { Accept: 'application/vnd.github+json' }
    });
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const data = await res.json();
    const tag = String(data.tag_name || '').trim();
    if (tag) els.forEach(el => { el.textContent = tag.startsWith('v') ? tag : 'v' + tag; });
    const assets = data.assets || [];
    const winA = assets.find(a => /\.exe$/i.test(a.name || ''));
    const linA = assets.find(a => /^volvid$/i.test(a.name || ''));
    const page = data.html_url || 'https://github.com/' + REPO + '/releases/latest';
    const dlWin = document.getElementById('dlWin');
    const dlLin = document.getElementById('dlLin');
    if (dlWin) dlWin.href = (winA && winA.browser_download_url) || page;
    if (dlLin) dlLin.href = (linA && linA.browser_download_url) || page;
  } catch (err) { /* keep fallback version */ }
}
loadVersion();

// First download button matches the visitor's OS (Linux → Volvid, else → Volvid.exe)
(function osFirst() {
  const box = document.querySelector('.btns');
  const dlWin = document.getElementById('dlWin');
  const dlLin = document.getElementById('dlLin');
  if (!box || !dlWin || !dlLin) return;
  const ghost = box.querySelector('.btn.ghost');
  const plat = String((navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '').toLowerCase();
  const first = plat.includes('linux') ? dlLin : dlWin;
  const second = first === dlLin ? dlWin : dlLin;
  box.prepend(first);
  if (ghost) box.appendChild(ghost);
  first.classList.add('primary'); first.classList.remove('outline');
  second.classList.add('outline'); second.classList.remove('primary');
})();

// RU / EN language switch for the whole site
const I18N = {
  ru: {
    'doc.title': 'Volvid — YouTube-загрузчик для терминала',
    'hero.tagline': 'Быстрый. Удобный. Клавиатурный.',
    'hero.title': 'Загружай видео и аудио<br>с YouTube <span class="blue">прямо в терминале.</span>',
    'hero.desc': 'Volvid — это быстрый и удобный TUI-интерфейс<br>для скачивания видео, аудио и превью<br>с YouTube.',
    'hero.dl': 'Скачать', 'hero.dl2': 'Скачать',
    'hero.forWin': 'для Windows', 'hero.forLin': 'для Linux',
    'hero.github': 'Смотреть на GitHub',
    'hero.release': 'Последний релиз:',
    'app.paste': 'Вставь ссылку на видео или плейлист YouTube',
    'app.target': 'Цель', 'app.dlLoc': 'Папка загрузки', 'app.recent': 'Текущая сессия',
    'app.noDl': '│ В этой сессии ещё не было загрузок.',
    'app.ok': 'ок', 'app.fail': 'ошибки',
    'app.cont': 'продолжить', 'app.search': 'поиск', 'app.folder': 'выбрать папку', 'app.open': 'открыть папку',
    'feat.title': 'Возможности',
    'feat.sub': 'Все, что нужно для удобной загрузки, в одном инструменте.',
    'f1t': 'Видео, аудио и превью',
    'f1d': 'Скачивайте видео, аудиодорожки и миниатюры через единый пошаговый TUI: от проверки обновлений до итога сессии.',
    'f2t': 'Пресеты качества',
    'f2d': 'Best и экономные пресеты видео со сканированием качества через yt-dlp. Аудио: MP3 320k и 192k, M4A/AAC Best, Opus Best, FLAC.',
    'f3t': 'Плейлисты',
    'f3d': 'Браузер плейлистов: выбор клавишей Space, всё сразу — клавишей A, ручные диапазоны — через /.',
    'f4t': 'Поиск на YouTube',
    'f4d': 'Поиск видео с главного экрана по Ctrl+G — ссылку вставлять не обязательно.',
    'f5t': 'Зависимости под контролем',
    'f5d': 'Проверка yt-dlp и ffmpeg на старте и обновление прямо в интерфейсе по Ctrl+U. Системные бинари в приоритете.',
    'f6t': 'Cookies и JS-runtime',
    'f6d': 'Автодетект кукисов браузера на Windows и Linux, проверка опционального node для обхода защиты.',
    'f7t': 'Папка загрузок',
    'f7d': 'Выбор папки внутри приложения по Ctrl+O и быстрое открытие. Свой путь — через VOLVID_DOWNLOADS_DIR.',
    'f8t': 'Итоги сессии',
    'f8d': 'Сводка после загрузок: история успехов и ошибок, счётчики ok и failed.',
    'q.title': 'Выбери качество',
    'q.move': 'движение', 'q.choose': 'выбрать', 'q.cont': 'продолжить', 'q.back': 'назад',
    'show.eyebrow': 'ПРОСТОЙ И ИНТУИТИВНЫЙ',
    'show.title': 'Чистый интерфейс.<br><span class="blue">Максимальный контроль.</span>',
    'show.desc': 'Volvid даёт вам всю мощь, не отвлекая лишним. Никаких окон, никаких переключений — только то, что нужно, именно тогда, когда нужно.',
    'c1': 'Управление с клавиатуры', 'c2': 'Быстрый и лёгкий', 'c3': 'Красивый и минималистичный TUI',
    'show.fmt': 'Форматы:',
    'foot.rel': 'Релизы', 'foot.src': 'Исходный код', 'foot.iss': 'Сообщить об ошибке',
    'foot.right': 'Открытый код &nbsp;•&nbsp; Лицензия MIT'
  },
  en: {
    'doc.title': 'Volvid — YouTube Downloader for the Terminal',
    'hero.tagline': 'Fast. Handy. Keyboard-driven.',
    'hero.title': 'Download video & audio<br>from YouTube <span class="blue">right in the terminal.</span>',
    'hero.desc': 'Volvid is a fast, handy TUI<br>for downloading video, audio & thumbnails<br>from YouTube.',
    'hero.dl': 'Download', 'hero.dl2': 'Download',
    'hero.forWin': 'for Windows', 'hero.forLin': 'for Linux',
    'hero.github': 'View on GitHub',
    'hero.release': 'Latest release:',
    'app.paste': 'Paste a YouTube video or playlist URL',
    'app.target': 'Target', 'app.dlLoc': 'Download location', 'app.recent': 'Recent session',
    'app.noDl': '│ No downloads yet in this session.',
    'app.ok': 'ok', 'app.fail': 'failed',
    'app.cont': 'continue', 'app.search': 'search', 'app.folder': 'choose folder', 'app.open': 'open folder',
    'feat.title': 'Features',
    'feat.sub': 'Everything you need for easy downloads, in one tool.',
    'f1t': 'Video, audio & thumbnails',
    'f1d': 'Download videos, audio tracks and thumbnails through one guided stage-based TUI: from update check to session summary.',
    'f2t': 'Quality presets',
    'f2d': 'Best and economy video presets with yt-dlp quality scan. Audio: MP3 320k & 192k, M4A/AAC Best, Opus Best, FLAC.',
    'f3t': 'Playlists',
    'f3d': 'Playlist browser: pick with Space, select all with A, manual ranges with /.',
    'f4t': 'YouTube search',
    'f4d': 'Search videos from the main screen with Ctrl+G — no link pasting required.',
    'f5t': 'Dependencies in check',
    'f5d': 'yt-dlp and ffmpeg check at startup, refresh right in the UI with Ctrl+U. System binaries preferred.',
    'f6t': 'Cookies & JS runtime',
    'f6d': 'Auto-detects browser cookies on Windows and Linux, checks the optional node runtime.',
    'f7t': 'Download folder',
    'f7d': 'Pick a folder inside the app with Ctrl+O and open it quickly. Custom path via VOLVID_DOWNLOADS_DIR.',
    'f8t': 'Session summary',
    'f8d': 'Post-download summary: success and failure history, ok and failed counters.',
    'q.title': 'Choose quality',
    'q.move': 'move', 'q.choose': 'choose', 'q.cont': 'continue', 'q.back': 'back',
    'show.eyebrow': 'SIMPLE AND INTUITIVE',
    'show.title': 'Clean interface.<br><span class="blue">Maximum control.</span>',
    'show.desc': 'Volvid gives you full power without distractions. No windows, no switching — only what you need, exactly when you need it.',
    'c1': 'Keyboard-driven', 'c2': 'Fast and lightweight', 'c3': 'Beautiful minimalist TUI',
    'show.fmt': 'Formats:',
    'foot.rel': 'Releases', 'foot.src': 'Source code', 'foot.iss': 'Report an issue',
    'foot.right': 'Open source &nbsp;•&nbsp; MIT License'
  }
};
let lang = 'ru';
try {
  const saved = localStorage.getItem('volvid-lang');
  if (saved === 'en' || saved === 'ru') lang = saved;
  else lang = (navigator.language || 'ru').toLowerCase().startsWith('en') ? 'en' : 'ru';
} catch (e) { /* private mode */ }
function setLang(l) {
  lang = l === 'en' ? 'en' : 'ru';
  try { localStorage.setItem('volvid-lang', lang); } catch (e) {}
  document.documentElement.lang = lang;
  document.title = I18N[lang]['doc.title'];
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const v = I18N[lang][el.dataset.i18n];
    if (v == null) return;
    if (el.hasAttribute('data-i18n-html')) el.innerHTML = v;
    else el.textContent = v;
  });
  document.querySelectorAll('#qList .unit').forEach(u => { u.textContent = lang === 'ru' ? 'МБ' : 'MB'; });
  document.querySelectorAll('.langseg button').forEach(b => b.classList.toggle('on', b.dataset.lang === lang));
}
document.querySelectorAll('.langseg button').forEach(b => b.addEventListener('click', () => setLang(b.dataset.lang)));
setLang(lang);

// Reveal on scroll (respects prefers-reduced-motion via CSS)
(function reveal() {
  const els = document.querySelectorAll('.features .f, .show-text, .showcase .app, .hero-right .app, .foot-top > div');
  if (!('IntersectionObserver' in window) || !els.length) return;
  els.forEach(el => el.classList.add('rv'));
  const io = new IntersectionObserver(entries => {
    entries.forEach(e => { if (e.isIntersecting) { e.target.classList.add('vis'); io.unobserve(e.target); } });
  }, { threshold: 0.12 });
  els.forEach(el => io.observe(el));
})();

