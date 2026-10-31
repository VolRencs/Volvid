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
    'doc.title': 'Volvid — YouTube-загрузчик для компьютера',
    'hero.title': 'Скачивайте видео, музыку и плейлисты с YouTube',
    'hero.desc': 'Загрузчик для Windows и Linux на Rust, egui, yt-dlp и ffmpeg.',
    'hero.dl': 'Скачать', 'hero.dl2': 'Скачать',
    'hero.forWin': 'для Windows', 'hero.forLin': 'для Linux',
    'hero.github': 'Смотреть на GitHub',
    'hero.release': 'Последний релиз:',
    'feat.title': 'Возможности',
    'feat.sub': 'Основные функции приложения.',
    'f1t': 'Видео, музыка и превью',
    'f1d': 'MP4, WebM, MKV, MP3, M4A, Opus и FLAC.',
    'f2t': 'Очередь загрузок',
    'f2d': 'Пауза, продолжение, отмена и повтор.',
    'f3t': 'Качество',
    'f3d': 'Доступные разрешения и примерный размер из yt-dlp.',
    'f4t': 'Плейлисты',
    'f4d': 'Фильтр, выбор всех и диапазоны: 1-5, 8, 12-.',
    'f5t': 'Ссылка и поиск',
    'f5d': 'Перетаскивание ссылки, вставка из буфера и поиск по Ctrl+G.',
    'f6t': 'Зависимости',
    'f6d': 'Статус yt-dlp, ffmpeg, node, cookies браузера и JS-runtime.',
    'f7t': 'Интерфейс',
    'f7d': 'Английский и русский язык, светлая и тёмная темы.',
    'f8t': 'История и дорожки',
    'f8d': 'История загрузок, субтитры и аудиодорожки.',
    'show.eyebrow': 'ИНТЕРФЕЙС',
    'show.title': 'Работа с плейлистами',
    'show.desc': 'Фильтр по названию, выбор всех, диапазоны и переход к настройкам загрузки.',
    'c1': 'Фильтр и диапазоны', 'c2': 'Субтитры и аудиодорожки', 'c3': 'История загрузок',
    'show.fmt': 'Форматы:',
    'foot.rel': 'Релизы', 'foot.src': 'Исходный код', 'foot.iss': 'Сообщить об ошибке',
    'foot.right': 'Открытый код &nbsp;•&nbsp; Лицензия GPL-3.0'
  },
  en: {
    'doc.title': 'Volvid — YouTube Downloader for Desktop',
    'hero.title': 'Download video, music and playlists from YouTube',
    'hero.desc': 'Windows and Linux downloader built with Rust, egui, yt-dlp and ffmpeg.',
    'hero.dl': 'Download', 'hero.dl2': 'Download',
    'hero.forWin': 'for Windows', 'hero.forLin': 'for Linux',
    'hero.github': 'View on GitHub',
    'hero.release': 'Latest release:',
    'feat.title': 'Features',
    'feat.sub': 'Main features.',
    'f1t': 'Video, music & thumbnails',
    'f1d': 'MP4, WebM, MKV, MP3, M4A, Opus and FLAC.',
    'f2t': 'Download queue',
    'f2d': 'Pause, resume, cancel and retry downloads.',
    'f3t': 'Quality',
    'f3d': 'Available resolutions and estimated sizes from yt-dlp.',
    'f4t': 'Playlists',
    'f4d': 'Filter, select all and ranges like 1-5, 8, 12-.',
    'f5t': 'Link and search',
    'f5d': 'Drag and drop, paste from clipboard or search with Ctrl+G.',
    'f6t': 'Dependencies',
    'f6d': 'Status of yt-dlp, ffmpeg, node, browser cookies and JS runtime.',
    'f7t': 'Interface',
    'f7d': 'English and Russian, light and dark themes.',
    'f8t': 'History & tracks',
    'f8d': 'Download history, subtitles and audio tracks.',
    'show.eyebrow': 'INTERFACE',
    'show.title': 'Playlist workflow',
    'show.desc': 'Filter by title, select all, apply ranges and continue to download settings.',
    'c1': 'Filter and ranges', 'c2': 'Subtitles and audio tracks', 'c3': 'Download history',
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
  const els = document.querySelectorAll('.features .f, .show-text, .showcase .shot-stage, .foot-top > div');
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

