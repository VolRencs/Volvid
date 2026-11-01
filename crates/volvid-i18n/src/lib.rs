use volvid_core::{
    DownloadMode, FragmentError, Locale, OutputProfile, QualityChoice, SelectionError,
    YTDLP_BEST_FORMAT, default_quality_choices,
};

macro_rules! define_strings {
    ($($field:ident: $en:expr, $ru:expr;)*) => {
        #[derive(Clone, Copy, Debug)]
        pub struct Strings {
            $(pub $field: &'static str,)*
        }

        static EN: Strings = Strings { $($field: $en,)* };
        static RU: Strings = Strings { $($field: $ru,)* };
    };
}

define_strings! {
    nav_new: "New download", "Новая загрузка";
    nav_queue: "Queue", "Очередь";
    nav_history: "History", "История";
    nav_settings: "Settings", "Настройки";

    home_title: "Download video & music from YouTube", "Скачивайте видео и музыку с YouTube";
    home_subtitle: "Paste a link, choose what you want, and Volvid handles the rest.", "Вставьте ссылку, выберите формат — остальное Volvid сделает сам.";
    url_placeholder: "https://www.youtube.com/watch?v=...", "https://www.youtube.com/watch?v=...";
    paste: "Paste", "Вставить";
    analyze: "Analyze", "Анализировать";
    search_youtube: "Search YouTube", "Поиск на YouTube";
    choose_folder: "Choose folder", "Выбрать папку";
    open_folder: "Open folder", "Открыть папку";
    download_location: "Download location", "Папка загрузки";
    recent: "Recent downloads", "Последние загрузки";
    no_history: "Nothing downloaded yet.", "В этой сессии ещё не было загрузок.";
    deps_missing: "Required dependencies are missing.", "Не хватает обязательных зависимостей.";
    drag_hint: "Drop a YouTube link here", "Перетащите ссылку YouTube сюда";
    clipboard_empty: "Clipboard is empty", "Буфер обмена пуст";

    theme: "Theme", "Тема";
    theme_system: "System", "Системная";
    theme_dark: "Dark", "Тёмная";
    theme_light: "Light", "Светлая";
    about: "About", "О программе";
    update_available: "Update available", "Доступно обновление";
    install_update: "Install update", "Установить обновление";
    later: "Later", "Позже";
    update_applied: "Update applied. Restart Volvid to use the new version.", "Обновление применено. Перезапустите Volvid.";
    close: "Close", "Закрыть";
    back: "Back", "Назад";
    continue_label: "Continue", "Продолжить";
    retry: "Retry", "Повторить";
    open: "Open", "Открыть";
    success: "Success", "Готово";

    deps_title: "Dependencies", "Зависимости";
    dep_required: "required", "обязательно";
    dep_optional: "optional", "опционально";
    dep_missing: "missing", "не найден";
    dep_available: "available", "доступен";
    dep_checking: "checking...", "проверяю...";
    dep_system: "system", "система";
    dep_managed: "managed", "в комплекте";
    dep_install: "Install", "Установить";
    dep_update: "Update", "Обновить";
    dep_refresh: "Refresh status", "Обновить статус";
    cookies: "Cookies", "Cookies";
    js_runtime: "JS runtime", "JS runtime";
    deps_settings_hint: "Open Settings → Dependencies to install them.", "Откройте «Настройки» → «Зависимости», чтобы установить их.";
    audio_original_hint: "Leave empty to keep the original audio only.", "Оставьте пустым, чтобы сохранить только оригинальное аудио.";
    environment: "Environment", "Окружение";
    table_name: "Name", "Название";
    table_duration: "Duration", "Длительность";
    downloading_count: "Downloading %d", "Загружается %d";
    waiting_count: "Waiting %d", "В ожидании %d";
    errors_count: "Errors %d", "Ошибок %d";
    clear_finished: "Clear completed", "Очистить завершённые";
    pause: "Pause", "Пауза";
    resume: "Resume", "Продолжить";
    pause_all: "Pause all", "Приостановить все";
    resume_all: "Resume all", "Продолжить все";
    paused: "Paused", "На паузе";
    cancelled: "Cancelled", "Отменено";
    remove: "Remove", "Удалить";
    queued_message: "Added to the download queue.", "Добавлено в очередь загрузок.";
    remaining: "left", "осталось";

    playlist_title: "Playlist", "Плейлист";
    videos_count: "%d videos", "%d видео";
    selected_count: "Selected %d / %d", "Выбрано %d / %d";
    filter_placeholder: "Filter by title", "Фильтр по названию";
    select_all: "Select all", "Выбрать все";
    select_none: "Clear", "Снять выбор";
    apply_range: "Apply range", "Применить диапазон";
    range_placeholder: "1-5, 8, 12-", "1-5, 8, 12-";
    loading_playlist: "Loading playlist…", "Загружаю плейлист…";
    playlist_empty: "Playlist is empty or unavailable.", "Плейлист пуст или недоступен.";
    video_title_fmt: "Video %d", "Видео %d";
    mixed_title: "Video and playlist in one link", "В ссылке есть видео и плейлист";
    mixed_hint: "What would you like to download?", "Что скачать?";
    video_only: "This video only", "Только это видео";
    full_playlist: "Full playlist", "Весь плейлист";
    no_selection: "Select at least one video.", "Выберите хотя бы одно видео.";

    configure_title: "Download settings", "Параметры загрузки";
    mode_video: "Video", "Видео";
    mode_audio: "Audio", "Аудио";
    mode_thumbnail: "Thumbnail", "Превью";
    quality_title: "Quality", "Качество";
    best_quality: "Best quality (HD · 4K)", "Лучшее качество (HD · 4K)";
    economy_quality: "Economy quality (360p)", "Экономичное качество (360p)";
    format_title: "Video format", "Формат видео";
    audio_title: "Audio format", "Формат аудио";
    thumbnail_title: "Thumbnail", "Превью";
    thumbnail_hint: "YouTube thumbnail in the best available resolution.", "Превью YouTube в максимальном доступном качестве.";
    thumbnail_failed: "Could not load the preview.", "Не удалось загрузить превью.";
    trim_title: "Trim", "Фрагмент";
    trim_playlist_unavailable: "Trimming is unavailable for playlists.", "Обрезка недоступна для плейлистов.";
    trim_enable: "Download a fragment", "Скачать фрагмент";
    from_url_timestamp: "From URL timestamp", "С таймкода из URL";
    custom_range: "Custom range", "Свой диапазон";
    full_media: "Full media", "Полностью";
    trim_hint: "Example: 1:00-2:30, 1:00- or 1:00+", "Пример: 1:00-2:30, 1:00- или 1:00+";
    duration: "Duration", "Длительность";
    audio_tracks_title: "Audio tracks", "Аудиодорожки";
    subtitles_title: "Subtitles", "Субтитры";
    no_audio_tracks: "No alternate audio tracks.", "Дополнительных аудиодорожек нет.";
    no_subtitles: "No subtitles available.", "Субтитры недоступны.";
    auto_subtitle: "auto", "авто";
    workers: "Parallel workers", "Параллельные потоки";
    output_folder: "Output folder", "Папка назначения";
    estimated_size: "Estimated size", "Примерный размер";
    download_button: "Download", "Скачать";
    loading_quality: "Scanning qualities…", "Сканирую качества…";
    loading_tracks: "Loading audio tracks and subtitles…", "Загружаю аудиодорожки и субтитры…";
    quality_failed: "Could not scan qualities; fallback presets are used.", "Не удалось просканировать качества; используются стандартные пресеты.";
    advanced: "Audio & subtitles", "Аудио и субтитры";

    video_original: "Original MP4", "Оригинал MP4";
    video_h264: "MP4 · H.264/AAC", "MP4 · H.264/AAC";
    video_h265: "MP4 · H.265/AAC", "MP4 · H.265/AAC";
    video_vp9: "WebM · VP9/Opus", "WebM · VP9/Opus";
    video_av1: "MKV · AV1/Opus", "MKV · AV1/Opus";
    video_mkv_copy: "MKV · copy streams", "MKV · копия потоков";

    audio_mp3_320: "MP3 320k", "MP3 320k";
    audio_mp3_192: "MP3 192k", "MP3 192k";
    audio_m4a_best: "M4A/AAC Best", "M4A/AAC Лучшее";
    audio_opus_best: "Opus Best", "Opus Лучшее";
    audio_flac: "FLAC Lossless", "FLAC Lossless";

    queue: "Queue", "Очередь";
    no_downloads: "No downloads in the queue.", "В очереди нет загрузок.";
    completed: "Completed", "Завершено";
    failed: "Failed", "Ошибки";
    queued: "Queued", "В очереди";
    downloading: "Downloading", "Загрузка";
    merging: "Merging video and audio", "Объединение видео и аудио";
    converting_audio: "Converting audio", "Конвертация аудио";
    thumbnail_proc: "Downloading thumbnail", "Загрузка превью";
    fallback_format: "Trying fallback format #%d: %s", "Пробую запасной формат #%d: %s";
    download_failed: "Download failed", "Загрузка не удалась";
    history_title: "History", "История";
    session_history: "Session history", "История сессии";
    clear_history: "Clear history", "Очистить историю";
    job_items: "%d items", "%d видео";

    search_title: "Search YouTube", "Поиск YouTube";
    search_prompt: "Enter a video title or keywords", "Введите название видео или ключевые слова";
    search_placeholder: "lofi hip hop mix", "lofi hip hop mix";
    searching: "Searching YouTube…", "Ищу на YouTube…";
    no_results: "Nothing found.", "Ничего не найдено.";
    query_empty: "Search query cannot be empty.", "Поисковый запрос не может быть пустым.";

    invalid_url: "Does not look like a YouTube URL", "Не похоже на YouTube-ссылку";
    empty_url: "URL cannot be empty", "Ссылка не может быть пустой";
    unsupported_url: "Unsupported YouTube URL", "Неподдерживаемая YouTube-ссылка";
    missing_ytdlp: "yt-dlp is required.", "Требуется yt-dlp.";
    missing_ffmpeg: "ffmpeg is required for this mode.", "Для этого режима требуется ffmpeg.";
    invalid_fragment: "Invalid fragment format.", "Неверный формат фрагмента.";
    fragment_out_of_range: "Range must stay within the video duration.", "Диапазон должен укладываться в длительность видео.";
    fragment_duration_required: "Trimming is unavailable because the video duration is unknown.", "Обрезка недоступна: длительность видео неизвестна.";
    fragment_bounds: "Start time must be less than end time.", "Начало должно быть меньше конца.";
    pick_folder_failed: "Failed to choose download folder", "Не удалось выбрать папку загрузки";
    open_folder_failed: "Failed to open folder", "Не удалось открыть папку";
    install_failed: "Dependency installation failed", "Не удалось установить зависимость";
    update_failed: "Update failed", "Не удалось обновить приложение";
    operation_cancelled: "Operation cancelled", "Операция отменена";
    config_save_failed: "Could not save settings", "Не удалось сохранить настройки";

    settings_title: "Settings", "Настройки";
    playlist_workers: "Playlist workers", "Потоков для плейлиста";
    logs: "Logs", "Логи";
    open_logs: "Open logs", "Открыть логи";
    config_dir: "Config directory", "Каталог конфигурации";
    downloads_dir: "Downloads directory", "Каталог загрузок";
    current_version: "Current version", "Текущая версия";
    latest_version: "Latest version", "Последняя версия";
    check_now: "Check now", "Проверить сейчас";
    updating: "Updating…", "Обновляю…";
    history_save_failed: "Could not save history", "Не удалось сохранить историю";
    paused_count: "Paused %d", "На паузе %d";
    failed_items: "%d failed", "%d с ошибкой";
    status_active: "active", "активно";
    status_not_found: "not found", "не найдено";
    status_no_profile: "no usable profile", "нет подходящего профиля";
    select_language: "Language", "Язык";
    appearance: "Appearance", "Внешний вид";
    behavior: "Behavior", "Поведение";
    paths: "Paths", "Пути";

    checking: "Checking…", "Проверяю…";
    loading: "Loading…", "Загрузка…";
    cancel: "Cancel", "Отмена";
}

pub fn strings(locale: Locale) -> &'static Strings {
    match locale {
        Locale::En => &EN,
        Locale::Ru => &RU,
    }
}

/// Translates the most common engine error messages that reach the UI.
pub fn engine_error_text(message: &str, locale: Locale) -> String {
    let s = strings(locale);
    let lower = message.to_ascii_lowercase();
    if lower.contains("yt-dlp is required") {
        s.missing_ytdlp.to_owned()
    } else if lower.contains("ffmpeg is required") {
        s.missing_ffmpeg.to_owned()
    } else if lower.contains("operation cancelled") {
        s.operation_cancelled.to_owned()
    } else if lower.contains("playlist empty or unavailable")
        || lower.contains("playlist is empty or unavailable")
    {
        s.playlist_empty.to_owned()
    } else if lower.contains("trimming is unavailable") {
        s.trim_playlist_unavailable.to_owned()
    } else if lower.contains("search returned no results") {
        s.no_results.to_owned()
    } else if lower.contains("download target is required") {
        s.empty_url.to_owned()
    } else if lower.contains("no download formats configured") {
        s.download_failed.to_owned()
    } else {
        message.to_owned()
    }
}

const BYTES_PER_TB: f64 = 1_099_511_627_776.0;
const BYTES_PER_GB: f64 = 1_073_741_824.0;
const BYTES_PER_MB: f64 = 1_048_576.0;

pub fn format_bytes(n: i64, locale: Locale) -> String {
    let ru = locale.is_ru();
    let n = n.max(0);
    if n as f64 >= BYTES_PER_TB {
        format!(
            "{:.2} {}",
            n as f64 / BYTES_PER_TB,
            if ru { "ТБ" } else { "TB" }
        )
    } else if n as f64 >= BYTES_PER_GB {
        format!(
            "{:.2} {}",
            n as f64 / BYTES_PER_GB,
            if ru { "ГБ" } else { "GB" }
        )
    } else if n as f64 >= BYTES_PER_MB {
        format!(
            "{:.1} {}",
            n as f64 / BYTES_PER_MB,
            if ru { "МБ" } else { "MB" }
        )
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

fn default_video_profile(locale: Locale) -> OutputProfile {
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
        SelectionError::Empty | SelectionError::None => s.no_selection.to_owned(),
        SelectionError::RangeOutOfBounds { start, end, max } => {
            format!("{start}-{end}: 1–{max}")
        }
        SelectionError::NumberOutOfBounds { value, max } => format!("{value}: 1–{max}"),
        SelectionError::InvalidToken { token } => format!("{token:?}"),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn formats_binary_byte_units() {
        assert_eq!(format_bytes(0, Locale::En), "0 B");
        assert_eq!(format_bytes(1024, Locale::En), "1 KB");
        assert_eq!(format_bytes(1024, Locale::Ru), "1 КБ");
        assert_eq!(format_bytes(1_048_576, Locale::En), "1.0 MB");
    }

    #[test]
    fn localizes_common_engine_errors() {
        assert_eq!(
            engine_error_text("yt-dlp is required", Locale::Ru),
            strings(Locale::Ru).missing_ytdlp
        );
        assert_eq!(
            engine_error_text("operation cancelled", Locale::En),
            strings(Locale::En).operation_cancelled
        );
        assert_eq!(
            engine_error_text("custom error", Locale::En),
            "custom error"
        );
    }
}
