use volvid_core::{
    DownloadMode, FragmentError, Locale, OutputProfile, QualityChoice, SelectionError,
    YTDLP_BEST_FORMAT, default_quality_choices,
};

#[derive(Clone, Copy, Debug)]
pub struct Strings {
    pub nav_new: &'static str,
    pub nav_queue: &'static str,
    pub nav_history: &'static str,
    pub nav_settings: &'static str,

    pub home_title: &'static str,
    pub home_subtitle: &'static str,
    pub url_placeholder: &'static str,
    pub paste: &'static str,
    pub analyze: &'static str,
    pub search_youtube: &'static str,
    pub choose_folder: &'static str,
    pub open_folder: &'static str,
    pub download_location: &'static str,
    pub recent: &'static str,
    pub no_history: &'static str,
    pub deps_missing: &'static str,
    pub drag_hint: &'static str,
    pub clipboard_empty: &'static str,

    pub language: &'static str,
    pub theme: &'static str,
    pub theme_system: &'static str,
    pub theme_dark: &'static str,
    pub theme_light: &'static str,
    pub about: &'static str,
    pub version: &'static str,
    pub check_updates: &'static str,
    pub update_available: &'static str,
    pub install_update: &'static str,
    pub later: &'static str,
    pub update_applied: &'static str,
    pub close: &'static str,
    pub back: &'static str,
    pub continue_label: &'static str,
    pub download: &'static str,
    pub retry: &'static str,
    pub open: &'static str,
    pub clear: &'static str,
    pub error: &'static str,
    pub warning: &'static str,
    pub success: &'static str,

    pub deps_title: &'static str,
    pub dep_required: &'static str,
    pub dep_optional: &'static str,
    pub dep_missing: &'static str,
    pub dep_available: &'static str,
    pub dep_checking: &'static str,
    pub dep_system: &'static str,
    pub dep_managed: &'static str,
    pub dep_install: &'static str,
    pub dep_update: &'static str,
    pub dep_refresh: &'static str,
    pub cookies: &'static str,
    pub js_runtime: &'static str,
    pub deps_settings_hint: &'static str,
    pub audio_original_hint: &'static str,
    pub environment: &'static str,
    pub table_name: &'static str,
    pub table_duration: &'static str,
    pub downloading_count: &'static str,
    pub waiting_count: &'static str,
    pub errors_count: &'static str,
    pub clear_finished: &'static str,
    pub pause: &'static str,
    pub resume: &'static str,
    pub pause_all: &'static str,
    pub resume_all: &'static str,
    pub paused: &'static str,
    pub cancelled: &'static str,
    pub remove: &'static str,
    pub queued_message: &'static str,
    pub remaining: &'static str,

    pub playlist_title: &'static str,
    pub videos_count: &'static str,
    pub selected_count: &'static str,
    pub filter_placeholder: &'static str,
    pub select_all: &'static str,
    pub select_none: &'static str,
    pub apply_range: &'static str,
    pub range_placeholder: &'static str,
    pub loading_playlist: &'static str,
    pub playlist_empty: &'static str,
    pub video_title_fmt: &'static str,
    pub mixed_title: &'static str,
    pub mixed_hint: &'static str,
    pub video_only: &'static str,
    pub full_playlist: &'static str,
    pub no_selection: &'static str,

    pub configure_title: &'static str,
    pub mode_video: &'static str,
    pub mode_audio: &'static str,
    pub mode_thumbnail: &'static str,
    pub quality_title: &'static str,
    pub best_quality: &'static str,
    pub economy_quality: &'static str,
    pub format_title: &'static str,
    pub audio_title: &'static str,
    pub thumbnail_title: &'static str,
    pub trim_title: &'static str,
    pub trim_enable: &'static str,
    pub from_url_timestamp: &'static str,
    pub custom_range: &'static str,
    pub full_media: &'static str,
    pub trim_hint: &'static str,
    pub duration: &'static str,
    pub audio_tracks_title: &'static str,
    pub subtitles_title: &'static str,
    pub no_audio_tracks: &'static str,
    pub no_subtitles: &'static str,
    pub auto_subtitle: &'static str,
    pub workers: &'static str,
    pub output_folder: &'static str,
    pub estimated_size: &'static str,
    pub download_button: &'static str,
    pub loading_quality: &'static str,
    pub loading_tracks: &'static str,
    pub quality_failed: &'static str,
    pub advanced: &'static str,

    pub video_original: &'static str,
    pub video_h264: &'static str,
    pub video_h265: &'static str,
    pub video_vp9: &'static str,
    pub video_av1: &'static str,
    pub video_mkv_copy: &'static str,

    pub audio_mp3_320: &'static str,
    pub audio_mp3_192: &'static str,
    pub audio_m4a_best: &'static str,
    pub audio_opus_best: &'static str,
    pub audio_flac: &'static str,

    pub queue: &'static str,
    pub no_downloads: &'static str,
    pub active: &'static str,
    pub completed: &'static str,
    pub failed: &'static str,
    pub queued: &'static str,
    pub speed: &'static str,
    pub elapsed: &'static str,
    pub cancel_download: &'static str,
    pub downloading: &'static str,
    pub merging: &'static str,
    pub converting_audio: &'static str,
    pub thumbnail_proc: &'static str,
    pub fallback_format: &'static str,
    pub download_failed: &'static str,
    pub history_title: &'static str,
    pub session_history: &'static str,
    pub clear_history: &'static str,
    pub job_items: &'static str,

    pub search_title: &'static str,
    pub search_prompt: &'static str,
    pub search_placeholder: &'static str,
    pub searching: &'static str,
    pub no_results: &'static str,
    pub query_empty: &'static str,

    pub invalid_url: &'static str,
    pub empty_url: &'static str,
    pub unsupported_url: &'static str,
    pub missing_ytdlp: &'static str,
    pub missing_ffmpeg: &'static str,
    pub invalid_fragment: &'static str,
    pub fragment_out_of_range: &'static str,
    pub fragment_duration_required: &'static str,
    pub fragment_bounds: &'static str,
    pub pick_folder_failed: &'static str,
    pub open_folder_failed: &'static str,
    pub install_failed: &'static str,
    pub update_failed: &'static str,
    pub operation_cancelled: &'static str,
    pub config_save_failed: &'static str,

    pub settings_title: &'static str,
    pub playlist_workers: &'static str,
    pub logs: &'static str,
    pub open_logs: &'static str,
    pub config_dir: &'static str,
    pub data_dir: &'static str,
    pub downloads_dir: &'static str,
    pub current_version: &'static str,
    pub latest_version: &'static str,
    pub check_now: &'static str,
    pub select_language: &'static str,
    pub appearance: &'static str,
    pub behavior: &'static str,
    pub paths: &'static str,

    pub checking: &'static str,
    pub loading: &'static str,
    pub cancel: &'static str,
}

pub static EN: Strings = Strings {
    nav_new: "New download",
    nav_queue: "Queue",
    nav_history: "History",
    nav_settings: "Settings",

    home_title: "Download video & music from YouTube",
    home_subtitle: "Paste a link, choose what you want, and Volvid handles the rest.",
    url_placeholder: "https://www.youtube.com/watch?v=...",
    paste: "Paste",
    analyze: "Analyze",
    search_youtube: "Search YouTube",
    choose_folder: "Choose folder",
    open_folder: "Open folder",
    download_location: "Download location",
    recent: "Recent downloads",
    no_history: "Nothing downloaded yet.",
    deps_missing: "Required dependencies are missing.",
    drag_hint: "Drop a YouTube link here",
    clipboard_empty: "Clipboard is empty",

    language: "Language",
    theme: "Theme",
    theme_system: "System",
    theme_dark: "Dark",
    theme_light: "Light",
    about: "About",
    version: "Version",
    check_updates: "Check for updates",
    update_available: "Update available",
    install_update: "Install update",
    later: "Later",
    update_applied: "Update applied. Restart Volvid to use the new version.",
    close: "Close",
    back: "Back",
    continue_label: "Continue",
    download: "Download",
    retry: "Retry",
    open: "Open",
    clear: "Clear",
    error: "Error",
    warning: "Warning",
    success: "Success",

    deps_title: "Dependencies",
    dep_required: "required",
    dep_optional: "optional",
    dep_missing: "missing",
    dep_available: "available",
    dep_checking: "checking...",
    dep_system: "system",
    dep_managed: "managed",
    dep_install: "Install",
    dep_update: "Update",
    dep_refresh: "Refresh status",
    cookies: "Cookies",
    js_runtime: "JS runtime",
    deps_settings_hint: "Open Settings → Dependencies to install them.",
    audio_original_hint: "Leave empty to keep the original audio only.",
    environment: "Environment",
    table_name: "Name",
    table_duration: "Duration",
    downloading_count: "Downloading %d",
    waiting_count: "Waiting %d",
    errors_count: "Errors %d",
    clear_finished: "Clear completed",
    pause: "Pause",
    resume: "Resume",
    pause_all: "Pause all",
    resume_all: "Resume all",
    paused: "Paused",
    cancelled: "Cancelled",
    remove: "Remove",
    queued_message: "Added to the download queue.",
    remaining: "left",

    playlist_title: "Playlist",
    videos_count: "%d videos",
    selected_count: "Selected %d / %d",
    filter_placeholder: "Filter by title",
    select_all: "Select all",
    select_none: "Clear",
    apply_range: "Apply range",
    range_placeholder: "1-5, 8, 12-",
    loading_playlist: "Loading playlist…",
    playlist_empty: "Playlist is empty or unavailable.",
    video_title_fmt: "Video %d",
    mixed_title: "Video and playlist in one link",
    mixed_hint: "What would you like to download?",
    video_only: "This video only",
    full_playlist: "Full playlist",
    no_selection: "Select at least one video.",

    configure_title: "Download settings",
    mode_video: "Video",
    mode_audio: "Audio",
    mode_thumbnail: "Thumbnail",
    quality_title: "Quality",
    best_quality: "Best quality (HD · 4K)",
    economy_quality: "Economy quality (360p)",
    format_title: "Video format",
    audio_title: "Audio format",
    thumbnail_title: "Thumbnail",
    trim_title: "Trim",
    trim_enable: "Download a fragment",
    from_url_timestamp: "From URL timestamp",
    custom_range: "Custom range",
    full_media: "Full media",
    trim_hint: "Example: 1:00-2:30, 1:00- or 1:00+",
    duration: "Duration",
    audio_tracks_title: "Audio tracks",
    subtitles_title: "Subtitles",
    no_audio_tracks: "No alternate audio tracks.",
    no_subtitles: "No subtitles available.",
    auto_subtitle: "auto",
    workers: "Parallel workers",
    output_folder: "Output folder",
    estimated_size: "Estimated size",
    download_button: "Download",
    loading_quality: "Scanning qualities…",
    loading_tracks: "Loading audio tracks and subtitles…",
    quality_failed: "Could not scan qualities; fallback presets are used.",
    advanced: "Audio & subtitles",

    video_original: "Original MP4",
    video_h264: "MP4 · H.264/AAC",
    video_h265: "MP4 · H.265/AAC",
    video_vp9: "WebM · VP9/Opus",
    video_av1: "MKV · AV1/Opus",
    video_mkv_copy: "MKV · copy streams",

    audio_mp3_320: "MP3 320k",
    audio_mp3_192: "MP3 192k",
    audio_m4a_best: "M4A/AAC Best",
    audio_opus_best: "Opus Best",
    audio_flac: "FLAC Lossless",

    queue: "Queue",
    no_downloads: "No downloads in the queue.",
    active: "Active",
    completed: "Completed",
    failed: "Failed",
    queued: "Queued",
    speed: "Speed",
    elapsed: "Elapsed",
    cancel_download: "Cancel",
    downloading: "Downloading",
    merging: "Merging video and audio",
    converting_audio: "Converting audio",
    thumbnail_proc: "Downloading thumbnail",
    fallback_format: "Trying fallback format #%d: %s",
    download_failed: "Download failed",
    history_title: "History",
    session_history: "Session history",
    clear_history: "Clear history",
    job_items: "%d items",

    search_title: "Search YouTube",
    search_prompt: "Enter a video title or keywords",
    search_placeholder: "lofi hip hop mix",
    searching: "Searching YouTube…",
    no_results: "Nothing found.",
    query_empty: "Search query cannot be empty.",

    invalid_url: "Does not look like a YouTube URL",
    empty_url: "URL cannot be empty",
    unsupported_url: "Unsupported YouTube URL",
    missing_ytdlp: "yt-dlp is required.",
    missing_ffmpeg: "ffmpeg is required for this mode.",
    invalid_fragment: "Invalid fragment format.",
    fragment_out_of_range: "Range must stay within the video duration.",
    fragment_duration_required: "Trimming is unavailable because the video duration is unknown.",
    fragment_bounds: "Start time must be less than end time.",
    pick_folder_failed: "Failed to choose download folder",
    open_folder_failed: "Failed to open folder",
    install_failed: "Dependency installation failed",
    update_failed: "Update failed",
    operation_cancelled: "Operation cancelled",
    config_save_failed: "Could not save settings",

    settings_title: "Settings",
    playlist_workers: "Playlist workers",
    logs: "Logs",
    open_logs: "Open logs",
    config_dir: "Config directory",
    data_dir: "Data directory",
    downloads_dir: "Downloads directory",
    current_version: "Current version",
    latest_version: "Latest version",
    check_now: "Check now",
    select_language: "Language",
    appearance: "Appearance",
    behavior: "Behavior",
    paths: "Paths",

    checking: "Checking…",
    loading: "Loading…",
    cancel: "Cancel",
};

pub static RU: Strings = Strings {
    nav_new: "Новая загрузка",
    nav_queue: "Очередь",
    nav_history: "История",
    nav_settings: "Настройки",

    home_title: "Скачивайте видео и музыку с YouTube",
    home_subtitle: "Вставьте ссылку, выберите формат — остальное Volvid сделает сам.",
    url_placeholder: "https://www.youtube.com/watch?v=...",
    paste: "Вставить",
    analyze: "Анализировать",
    search_youtube: "Поиск на YouTube",
    choose_folder: "Выбрать папку",
    open_folder: "Открыть папку",
    download_location: "Папка загрузки",
    recent: "Последние загрузки",
    no_history: "В этой сессии ещё не было загрузок.",
    deps_missing: "Не хватает обязательных зависимостей.",
    drag_hint: "Перетащите ссылку YouTube сюда",
    clipboard_empty: "Буфер обмена пуст",

    language: "Язык",
    theme: "Тема",
    theme_system: "Системная",
    theme_dark: "Тёмная",
    theme_light: "Светлая",
    about: "О программе",
    version: "Версия",
    check_updates: "Проверить обновления",
    update_available: "Доступно обновление",
    install_update: "Установить обновление",
    later: "Позже",
    update_applied: "Обновление применено. Перезапустите Volvid.",
    close: "Закрыть",
    back: "Назад",
    continue_label: "Продолжить",
    download: "Скачать",
    retry: "Повторить",
    open: "Открыть",
    clear: "Очистить",
    error: "Ошибка",
    warning: "Внимание",
    success: "Готово",

    deps_title: "Зависимости",
    dep_required: "обязательно",
    dep_optional: "опционально",
    dep_missing: "не найден",
    dep_available: "доступен",
    dep_checking: "проверяю...",
    dep_system: "система",
    dep_managed: "в комплекте",
    dep_install: "Установить",
    dep_update: "Обновить",
    dep_refresh: "Обновить статус",
    cookies: "Cookies",
    js_runtime: "JS runtime",
    deps_settings_hint: "Откройте «Настройки» → «Зависимости», чтобы установить их.",
    audio_original_hint: "Оставьте пустым, чтобы сохранить только оригинальное аудио.",
    environment: "Окружение",
    table_name: "Название",
    table_duration: "Длительность",
    downloading_count: "Загружается %d",
    waiting_count: "В ожидании %d",
    errors_count: "Ошибок %d",
    clear_finished: "Очистить завершённые",
    pause: "Пауза",
    resume: "Продолжить",
    pause_all: "Приостановить все",
    resume_all: "Продолжить все",
    paused: "На паузе",
    cancelled: "Отменено",
    remove: "Удалить",
    queued_message: "Добавлено в очередь загрузок.",
    remaining: "осталось",

    playlist_title: "Плейлист",
    videos_count: "%d видео",
    selected_count: "Выбрано %d / %d",
    filter_placeholder: "Фильтр по названию",
    select_all: "Выбрать все",
    select_none: "Снять выбор",
    apply_range: "Применить диапазон",
    range_placeholder: "1-5, 8, 12-",
    loading_playlist: "Загружаю плейлист…",
    playlist_empty: "Плейлист пуст или недоступен.",
    video_title_fmt: "Видео %d",
    mixed_title: "В ссылке есть видео и плейлист",
    mixed_hint: "Что скачать?",
    video_only: "Только это видео",
    full_playlist: "Весь плейлист",
    no_selection: "Выберите хотя бы одно видео.",

    configure_title: "Параметры загрузки",
    mode_video: "Видео",
    mode_audio: "Аудио",
    mode_thumbnail: "Превью",
    quality_title: "Качество",
    best_quality: "Лучшее качество (HD · 4K)",
    economy_quality: "Экономичное качество (360p)",
    format_title: "Формат видео",
    audio_title: "Формат аудио",
    thumbnail_title: "Превью",
    trim_title: "Фрагмент",
    trim_enable: "Скачать фрагмент",
    from_url_timestamp: "С таймкода из URL",
    custom_range: "Свой диапазон",
    full_media: "Полностью",
    trim_hint: "Пример: 1:00-2:30, 1:00- или 1:00+",
    duration: "Длительность",
    audio_tracks_title: "Аудиодорожки",
    subtitles_title: "Субтитры",
    no_audio_tracks: "Дополнительных аудиодорожек нет.",
    no_subtitles: "Субтитры недоступны.",
    auto_subtitle: "авто",
    workers: "Параллельные потоки",
    output_folder: "Папка назначения",
    estimated_size: "Примерный размер",
    download_button: "Скачать",
    loading_quality: "Сканирую качества…",
    loading_tracks: "Загружаю аудиодорожки и субтитры…",
    quality_failed: "Не удалось просканировать качества; используются стандартные пресеты.",
    advanced: "Аудио и субтитры",

    video_original: "Оригинал MP4",
    video_h264: "MP4 · H.264/AAC",
    video_h265: "MP4 · H.265/AAC",
    video_vp9: "WebM · VP9/Opus",
    video_av1: "MKV · AV1/Opus",
    video_mkv_copy: "MKV · копия потоков",

    audio_mp3_320: "MP3 320k",
    audio_mp3_192: "MP3 192k",
    audio_m4a_best: "M4A/AAC Лучшее",
    audio_opus_best: "Opus Лучшее",
    audio_flac: "FLAC Lossless",

    queue: "Очередь",
    no_downloads: "В очереди нет загрузок.",
    active: "Активно",
    completed: "Завершено",
    failed: "Ошибки",
    queued: "В очереди",
    speed: "Скорость",
    elapsed: "Прошло",
    cancel_download: "Отменить",
    downloading: "Загрузка",
    merging: "Объединение видео и аудио",
    converting_audio: "Конвертация аудио",
    thumbnail_proc: "Загрузка превью",
    fallback_format: "Пробую запасной формат #%d: %s",
    download_failed: "Загрузка не удалась",
    history_title: "История",
    session_history: "История сессии",
    clear_history: "Очистить историю",
    job_items: "%d видео",

    search_title: "Поиск YouTube",
    search_prompt: "Введите название видео или ключевые слова",
    search_placeholder: "lofi hip hop mix",
    searching: "Ищу на YouTube…",
    no_results: "Ничего не найдено.",
    query_empty: "Поисковый запрос не может быть пустым.",

    invalid_url: "Не похоже на YouTube-ссылку",
    empty_url: "Ссылка не может быть пустой",
    unsupported_url: "Неподдерживаемая YouTube-ссылка",
    missing_ytdlp: "Требуется yt-dlp.",
    missing_ffmpeg: "Для этого режима требуется ffmpeg.",
    invalid_fragment: "Неверный формат фрагмента.",
    fragment_out_of_range: "Диапазон должен укладываться в длительность видео.",
    fragment_duration_required: "Обрезка недоступна: длительность видео неизвестна.",
    fragment_bounds: "Начало должно быть меньше конца.",
    pick_folder_failed: "Не удалось выбрать папку загрузки",
    open_folder_failed: "Не удалось открыть папку",
    install_failed: "Не удалось установить зависимость",
    update_failed: "Не удалось обновить приложение",
    operation_cancelled: "Операция отменена",
    config_save_failed: "Не удалось сохранить настройки",

    settings_title: "Настройки",
    playlist_workers: "Потоков для плейлиста",
    logs: "Логи",
    open_logs: "Открыть логи",
    config_dir: "Каталог конфигурации",
    data_dir: "Каталог данных",
    downloads_dir: "Каталог загрузок",
    current_version: "Текущая версия",
    latest_version: "Последняя версия",
    check_now: "Проверить сейчас",
    select_language: "Язык",
    appearance: "Внешний вид",
    behavior: "Поведение",
    paths: "Пути",

    checking: "Проверяю…",
    loading: "Загрузка…",
    cancel: "Отмена",
};

pub fn strings(locale: Locale) -> &'static Strings {
    match locale {
        Locale::En => &EN,
        Locale::Ru => &RU,
    }
}

pub fn format_bytes(n: i64, locale: Locale) -> String {
    let ru = locale.is_ru();
    let n = n.max(0);
    const TB: f64 = 1_099_511_627_776.0;
    const GB: f64 = 1_073_741_824.0;
    const MB: f64 = 1_048_576.0;
    if n as f64 >= TB {
        format!("{:.2} {}", n as f64 / TB, if ru { "ТБ" } else { "TB" })
    } else if n as f64 >= GB {
        format!("{:.2} {}", n as f64 / GB, if ru { "ГБ" } else { "GB" })
    } else if n as f64 >= MB {
        format!("{:.1} {}", n as f64 / MB, if ru { "МБ" } else { "MB" })
    } else if n >= 1024 {
        format!("{} {}", n / 1024, if ru { "КБ" } else { "KB" })
    } else {
        format!("{n} {}", if ru { "Б" } else { "B" })
    }
}

pub fn format_speed(bytes_per_sec: i64, locale: Locale) -> String {
    format!(
        "{}{}",
        format_bytes(bytes_per_sec, locale),
        if locale.is_ru() { "/с" } else { "/s" }
    )
}

pub fn format_elapsed(seconds: i64, locale: Locale) -> String {
    let seconds = seconds.max(0);
    let h = seconds / 3600;
    let m = (seconds % 3600) / 60;
    let s = seconds % 60;
    if locale.is_ru() {
        if h > 0 {
            format!("{h} ч {m:02} мин {s:02} с")
        } else if m > 0 {
            format!("{m} мин {s:02} с")
        } else {
            format!("{s} с")
        }
    } else if h > 0 {
        format!("{h}h {m:02}m {s:02}s")
    } else if m > 0 {
        format!("{m}m {s:02}s")
    } else {
        format!("{s}s")
    }
}

pub fn quality_label(choice: &QualityChoice, locale: Locale) -> String {
    let s = strings(locale);
    let mut label = if choice.best {
        s.best_quality.to_owned()
    } else if choice.worst {
        s.economy_quality.to_owned()
    } else {
        let mut label = format!("{}p", choice.height);
        if choice.total > 1 {
            label.push_str(&format!("  ·  {}/{}", choice.available, choice.total));
        }
        label
    };
    if choice.size_bytes > 0 {
        label.push_str("  ·  ~");
        label.push_str(&format_bytes(choice.size_bytes, locale));
    }
    label
}

pub fn quality_profile(choice: &QualityChoice, locale: Locale) -> OutputProfile {
    OutputProfile {
        label: quality_label(choice, locale),
        mode: DownloadMode::Video,
        video_fmt_chain: choice.fmt_chain.clone(),
        video_fmt_labels: choice.fmt_labels.clone(),
        ..OutputProfile::default()
    }
}

pub fn default_video_profile(locale: Locale) -> OutputProfile {
    let s = strings(locale);
    OutputProfile {
        label: s.best_quality.to_owned(),
        mode: DownloadMode::Video,
        video_fmt_chain: vec![YTDLP_BEST_FORMAT.to_owned(), "best".to_owned()],
        ..OutputProfile::default()
    }
}

#[allow(clippy::too_many_arguments)]
fn with_video_output(
    mut base: OutputProfile,
    label: &str,
    container: &str,
    video_codec: &str,
    crf: &str,
    audio_codec: &str,
    audio_bitrate: &str,
    remux_only: bool,
) -> OutputProfile {
    if !base.label.is_empty() {
        base.label.push_str(" · ");
    }
    base.label.push_str(label);
    base.video_container = container.to_owned();
    base.video_codec = video_codec.to_owned();
    base.video_crf = crf.to_owned();
    base.audio_codec = audio_codec.to_owned();
    base.audio_bitrate = audio_bitrate.to_owned();
    base.remux_only = remux_only;
    base
}

pub fn video_output_profiles(base: OutputProfile, locale: Locale) -> Vec<OutputProfile> {
    let s = strings(locale);
    let mut base = base;
    if base.mode != DownloadMode::Video {
        base = default_video_profile(locale);
    }
    if base.label.trim().is_empty() {
        base.label = s.best_quality.to_owned();
    }
    if base.video_fmt_chain.is_empty() {
        base.video_fmt_chain = vec![YTDLP_BEST_FORMAT.to_owned(), "best".to_owned()];
    }
    vec![
        with_video_output(base.clone(), s.video_original, "mp4", "", "", "", "", false),
        with_video_output(
            base.clone(),
            s.video_h264,
            "mp4",
            "libx264",
            "20",
            "aac",
            "192k",
            false,
        ),
        with_video_output(
            base.clone(),
            s.video_h265,
            "mp4",
            "libx265",
            "24",
            "aac",
            "192k",
            false,
        ),
        with_video_output(
            base.clone(),
            s.video_vp9,
            "webm",
            "libvpx-vp9",
            "31",
            "libopus",
            "160k",
            false,
        ),
        with_video_output(
            base.clone(),
            s.video_av1,
            "mkv",
            "libsvtav1",
            "35",
            "libopus",
            "160k",
            false,
        ),
        with_video_output(base, s.video_mkv_copy, "mkv", "", "", "", "", true),
    ]
}

pub fn audio_output_profiles(locale: Locale) -> Vec<OutputProfile> {
    let s = strings(locale);
    [
        (s.audio_mp3_320, "mp3", "320K"),
        (s.audio_mp3_192, "mp3", "192K"),
        (s.audio_m4a_best, "m4a", "0"),
        (s.audio_opus_best, "opus", "0"),
        (s.audio_flac, "flac", "0"),
    ]
    .into_iter()
    .map(|(label, format, quality)| OutputProfile {
        label: label.to_owned(),
        mode: DownloadMode::Audio,
        audio_format: format.to_owned(),
        audio_quality: quality.to_owned(),
        ..OutputProfile::default()
    })
    .collect()
}

pub fn thumbnail_profile(locale: Locale) -> OutputProfile {
    OutputProfile {
        label: strings(locale).mode_thumbnail.to_owned(),
        mode: DownloadMode::Thumbnail,
        ..OutputProfile::default()
    }
}

pub fn default_quality(locale: Locale) -> Vec<QualityChoice> {
    let mut choices = default_quality_choices();
    if let Some(first) = choices.first_mut() {
        first.fmt_chain = vec![YTDLP_BEST_FORMAT.to_owned()];
        first.fmt_labels = vec![strings(locale).best_quality.to_owned()];
    }
    choices
}

pub fn fragment_error_text(err: FragmentError, media_duration: i64, locale: Locale) -> String {
    let s = strings(locale);
    match err {
        FragmentError::OutOfRange => {
            if media_duration > 0 {
                format!(
                    "{} ({})",
                    s.fragment_out_of_range,
                    volvid_core::format_clock(media_duration)
                )
            } else {
                s.fragment_duration_required.to_owned()
            }
        }
        FragmentError::DurationRequired => s.fragment_duration_required.to_owned(),
        FragmentError::Bounds => s.fragment_bounds.to_owned(),
        FragmentError::Format => s.invalid_fragment.to_owned(),
    }
}

pub fn selection_error_text(err: SelectionError, locale: Locale) -> String {
    let s = strings(locale);
    match err {
        SelectionError::Empty => s.no_selection.to_owned(),
        SelectionError::RangeOutOfBounds { start, end, max } => {
            format!("{start}-{end}: 1–{max}")
        }
        SelectionError::NumberOutOfBounds { value, max } => format!("{value}: 1–{max}"),
        SelectionError::InvalidToken { token } => format!("{token:?}"),
        SelectionError::None => s.no_selection.to_owned(),
    }
}
