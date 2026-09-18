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
const idle = window.requestIdleCallback || (fn => setTimeout(fn, 1200));
idle(loadVersion);

// RU / EN language switch for the whole site
const I18N = {
  ru: {
    'doc.title': 'Volvid — YouTube-загрузчик для терминала',
    'hero.title': 'Скачивай видео и музыку с YouTube <span class="blue">прямо из терминала.</span>',
    'hero.desc': 'Volvid — лёгкий TUI-загрузчик на yt-dlp и ffmpeg. Вставил ссылку, выбрал качество, получил файл — без браузера и лишних кликов.',
    'hero.dl': 'Скачать', 'hero.dl2': 'Скачать',
    'hero.forWin': 'для Windows', 'hero.forLin': 'для Linux',
    'hero.github': 'Смотреть на GitHub',
    'hero.release': 'Последний релиз:',
    'app.paste': 'Вставь ссылку на видео или плейлист YouTube',
    'app.target': 'Источник', 'app.dlLoc': 'Папка загрузки', 'app.path': '/home/volren/Видео/YouTube', 'app.recent': 'Текущая сессия',
    'app.noDl': 'В этой сессии ещё не было загрузок.',
    'app.ok': 'успешно', 'app.fail': 'ошибки',
    'app.cont': 'продолжить', 'app.search': 'поиск', 'app.folder': 'выбрать папку', 'app.open': 'открыть папку',
    'feat.title': 'Возможности',
    'feat.sub': 'Всё для загрузок — в одном приложении.',
    'f1t': 'Видео, аудио и превью',
    'f1d': 'Видео, аудиодорожки и обложки — всё в одном приложении.',
    'f2t': 'Пресеты качества',
    'f2d': 'От 144p до 4K, а также MP3 320k, M4A, Opus и FLAC.',
    'f3t': 'Плейлисты',
    'f3d': 'Отмечай нужные ролики клавишей Space, всё сразу — A, диапазоны — /.',
    'f4t': 'Поиск на YouTube',
    'f4d': 'Ищи видео по Ctrl+G, не выходя из приложения.',
    'f5t': 'Зависимости под контролем',
    'f5d': 'yt-dlp и ffmpeg проверяются при запуске и обновляются по Ctrl+U.',
    'f6t': 'Cookies и JS-runtime',
    'f6d': 'Автопоиск cookies браузера и node, чтобы обходить защиту.',
    'f8t': 'Итоги сессии',
    'f8d': 'История загрузок со счётчиками успехов и ошибок.',
    'f9t': 'Субтитры и дубляж',
    'f9d': 'Вшивает субтитры и аудиодорожки в видео, сохраняя языки.',
    'q.title': 'Выбери качество',
    'q.move': 'движение', 'q.choose': 'выбрать', 'q.cont': 'продолжить', 'q.back': 'назад',
    'show.eyebrow': 'ПРОСТО И БЫСТРО',
    'show.title': 'Всё под контролем. <span class="blue">И ничего лишнего.</span>',
    'show.desc': 'Каждый шаг — в одном экране: ссылка, качество, загрузка. Ни окон, ни вкладок, ни переключений.',
    'c1': 'Полное управление с клавиатуры', 'c2': 'Один лёгкий бинарник — без установки', 'c3': 'Аккуратный минималистичный TUI',
    'show.fmt': 'Форматы:',
    'foot.rel': 'Релизы', 'foot.src': 'Исходный код', 'foot.iss': 'Сообщить об ошибке',
    'foot.right': 'Открытый код &nbsp;•&nbsp; Лицензия GPL-3.0'
  },
  en: {
    'doc.title': 'Volvid — YouTube Downloader for the Terminal',
    'hero.title': 'Download video & music from YouTube <span class="blue">right from the terminal.</span>',
    'hero.desc': 'Volvid is a lightweight yt-dlp + ffmpeg downloader. Paste a link, pick a quality, get the file — no browser, no extra clicks.',
    'hero.dl': 'Download', 'hero.dl2': 'Download',
    'hero.forWin': 'for Windows', 'hero.forLin': 'for Linux',
    'hero.github': 'View on GitHub',
    'hero.release': 'Latest release:',
    'app.paste': 'Paste a YouTube video or playlist URL',
    'app.target': 'Target', 'app.dlLoc': 'Download location', 'app.path': '/home/volren/Videos/YouTube', 'app.recent': 'Recent session',
    'app.noDl': 'No downloads yet in this session.',
    'app.ok': 'ok', 'app.fail': 'failed',
    'app.cont': 'continue', 'app.search': 'search', 'app.folder': 'choose folder', 'app.open': 'open folder',
    'feat.title': 'Features',
    'feat.sub': 'Everything you need for downloads — in one app.',
    'f1t': 'Video, audio & thumbnails',
    'f1d': 'Video, audio tracks and thumbnails — all in one place.',
    'f2t': 'Quality presets',
    'f2d': '144p to 4K, plus MP3 320k, M4A, Opus and FLAC.',
    'f3t': 'Playlists',
    'f3d': 'Pick videos with Space, select all with A, set ranges with /.',
    'f4t': 'YouTube search',
    'f4d': 'Search YouTube with Ctrl+G without leaving the app.',
    'f5t': 'Dependencies in check',
    'f5d': 'yt-dlp and ffmpeg are checked on start and updated with Ctrl+U.',
    'f6t': 'Cookies & JS runtime',
    'f6d': 'Auto-detects browser cookies and the optional node runtime.',
    'f8t': 'Session summary',
    'f8d': 'Download history with success and failure counters.',
    'f9t': 'Subtitles & dubs',
    'f9d': 'Embeds subtitles and audio tracks into the video, keeping languages.',
    'q.title': 'Choose quality',
    'q.move': 'move', 'q.choose': 'choose', 'q.cont': 'continue', 'q.back': 'back',
    'show.eyebrow': 'SIMPLE AND FAST',
    'show.title': 'Everything under control. <span class="blue">And nothing extra.</span>',
    'show.desc': 'Every step on one screen: link, quality, download. No windows, no tabs, no switching.',
    'c1': 'Full keyboard control', 'c2': 'One lightweight binary, no install', 'c3': 'Clean minimalist TUI',
    'show.fmt': 'Formats:',
    'foot.rel': 'Releases', 'foot.src': 'Source code', 'foot.iss': 'Report an issue',
    'foot.right': 'Open source &nbsp;•&nbsp; GPL-3.0 License'
  }
};
let lang = 'en';
try {
  const saved = localStorage.getItem('volvid-lang');
  if (saved === 'en' || saved === 'ru') lang = saved;
  else lang = (navigator.language || 'en').toLowerCase().startsWith('ru') ? 'ru' : 'en';
} catch (e) { /* private mode */ }
const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
let firstPaint = true;
function applyLang() {
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
function setLang(l) {
  lang = l === 'en' ? 'en' : 'ru';
  const useVT = !firstPaint && !reduceMotion.matches && typeof document.startViewTransition === 'function';
  firstPaint = false;
  useVT ? document.startViewTransition(applyLang) : applyLang();
}
document.querySelectorAll('.langseg button').forEach(b => b.addEventListener('click', () => setLang(b.dataset.lang)));
setLang(lang);

// Scroll to top for logo buttons (no anchor ids on this small site)
document.querySelectorAll('[data-scroll-top]').forEach(b =>
  b.addEventListener('click', () => {
    window.scrollTo({ top: 0, behavior: reduceMotion.matches ? 'auto' : 'smooth' });
  })
);

// Reveal fallback: native scroll-driven animations handle this where supported
(function reveal() {
  if (reduceMotion.matches) return;
  if (CSS.supports && CSS.supports('animation-timeline: view()')) return;
  const els = document.querySelectorAll('.features .f, .show-text, .showcase .app, .foot-top > div');
  if (!('IntersectionObserver' in window) || !els.length) return;
  els.forEach((el, k) => {
    el.classList.add('rv');
    if (el.matches('.features .f')) el.style.transitionDelay = (k % 8) * 45 + 'ms';
  });
  const io = new IntersectionObserver(entries => {
    entries.forEach(e => {
      if (!e.isIntersecting) return;
      e.target.classList.add('vis');
      io.unobserve(e.target);
      setTimeout(() => { e.target.style.transitionDelay = ''; }, 700);
    });
  }, { threshold: 0.12 });
  els.forEach(el => io.observe(el));
})();

// Typing animation for URL (inside Target box, replaces placeholder)
const url = 'https://youtu.be/dQw4w9WgXcQ';
const typedEl = document.getElementById('typed');
const placeholderEl = document.getElementById('ph');
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

// Quality picker preview: auto-highlight only, not user-interactive
const q = [...document.querySelectorAll('#qList li')];
let qs = 0;
function paintQ() {
  q.forEach((li, k) => li.classList.toggle('sel', k === qs));
}
paintQ();
if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
  setInterval(() => { if (q.length) { qs = (qs + 1) % q.length; paintQ(); } }, 1800);
}

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

