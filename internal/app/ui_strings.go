package app

import "fmt"

type UIStrings struct {
	HeaderPowered string

	HelpUp, HelpDown, HelpEnter, HelpQuit, HelpDeps, HelpSpace, HelpAll, HelpSlash, HelpSearch, HelpBack string

	QBest, QEcon string

	SpinnerUpdate, SpinnerPlaylist, SpinnerQuality, SpinnerSearch, SpinnerFragment string

	UpdateAvail, CurrentVerShort string
	PlaylistMixWarn              string

	DepsUpdating, DepsRefreshing                          string
	UpdateAppliedWin, UpdateAppliedUnix, UpdateDonePrefix string
	HelpAnyKey, HelpExit, DepsOK, ErrPrefix               string

	AppSubtitle, TopBarDeps, TopBarDepsBusy string

	PasteURL, URLErrEmpty, URLErrBad, URLHints                                                            string
	SearchTitle, SearchPrompt, SearchPlaceholder, SearchErrEmpty, SearchErrFailed, SearchNoResults        string
	FragmentTitle, FragmentHint, FragmentFromURLFmt, FragmentInputTitle, FragmentInputPrompt              string
	FragmentInputHint, FragmentInputHintWithDurationFmt, FragmentInputBadFormat, FragmentInputBadRange    string
	FragmentDurationFmt, FragmentUnavailable, FragmentInputOutOfBoundsFmt, FragmentURLStartOutOfBoundsFmt string

	PlVideosFmt, PlEnterNums, PlSelectedFmt string
	ErrPickOne                              string

	PlInputPlaceholder                                              string
	PlParseEmpty, PlParseRange, PlParseNum, PlParseBad, PlParseNone string
	PlEmptyPlaylist, PlTimeout                                      string
	VideoTitleFmt                                                   string

	ParallelFmt, QualityTitle, AudioTitle, WorkersQueuedFmt string

	ModeTitle, ModeVideo, ModeAudio, ModeThumbnail                                 string
	AudioMP3320, AudioMP3192, AudioM4ABest, AudioOpusBest, AudioFLAC, OutThumbnail string

	Downloading, PlaylistBarFmt, QueueFmt, Waiting, ErrSlot, MergeProc, MP3Proc, ThumbProc string

	SummaryOK, SummaryFail, SummaryPlaylistTitle, SessionHist, SuccessFmt string

	MenuUpdateY, MenuUpdateN, MenuVidOnly, MenuOpenPl string
	MenuAgainY, MenuAgainN, WorkerSeq, WorkerNFmt     string
	MenuFullVideo, MenuFromURLStart, MenuManualRange  string

	DepLabelFmt string
	LangTab     string

	FallbackFmt string
	PlaylistTag string
}

var strEN = UIStrings{
	HeaderPowered: "v%s   powered by yt-dlp",

	HelpUp: "up", HelpDown: "down", HelpEnter: "enter", HelpQuit: "quit",
	HelpDeps: "update deps", HelpSpace: "space", HelpAll: "all", HelpSlash: "numbers",
	HelpSearch: "search", HelpBack: "back",

	QBest: "▲ Best (HD·4K)", QEcon: "▼ Economy (360p)",

	SpinnerUpdate: "  Checking for updates…", SpinnerPlaylist: "  Loading playlist…", SpinnerQuality: "  Scanning qualities…", SpinnerSearch: "  Searching YouTube…", SpinnerFragment: "  Detecting video duration…",

	UpdateAvail: "  ✔  New version ", CurrentVerShort: "  (current %s)",
	PlaylistMixWarn: "  ⚠️ URL has both a video and a playlist",

	DepsUpdating:      "Updating dependencies…",
	DepsRefreshing:    "Refreshing dependency status…",
	UpdateAppliedWin:  "  The app will close. The update bat will replace the exe in the background.",
	UpdateAppliedUnix: "  Binary replaced. Restart the app.",
	UpdateDonePrefix:  "  ✔  Update applied — ",
	HelpAnyKey:        "any key", HelpExit: "exit", DepsOK: "  ✔  Dependencies updated", ErrPrefix: "  ✘  Error: ",

	AppSubtitle: "  ·  Video / Audio Downloader",
	TopBarDeps:  "↻ update dependencies", TopBarDepsBusy: " updating…",

	PasteURL: "  Paste a video or playlist URL", URLErrEmpty: "URL cannot be empty",
	URLErrBad: "Does not look like a YouTube URL", URLHints: "\nyoutube.com/watch   youtube.com/playlist   youtu.be/",
	SearchTitle: "  Search YouTube", SearchPrompt: "  Enter a video title or keywords",
	SearchPlaceholder: "lofi hip hop mix", SearchErrEmpty: "Search query cannot be empty",
	SearchErrFailed: "Search failed", SearchNoResults: "Nothing found",
	FragmentTitle: "  Fragment", FragmentHint: "  Optional for single download in video or audio mode",
	FragmentFromURLFmt: "From URL timestamp (%s) to end", FragmentInputTitle: "  Enter fragment range",
	FragmentInputPrompt: "  Format: mm:ss-hh:mm:ss or hh:mm:ss-hh:mm:ss",
	FragmentInputHint:   "Example: 1:00-2:30", FragmentInputHintWithDurationFmt: "Example: 1:00-2:30\nVideo duration: %s\nEnd cannot exceed the video duration.",
	FragmentInputBadFormat: "Invalid time format", FragmentInputBadRange: "Start time must be less than end time",
	FragmentDurationFmt: "Video duration: %s", FragmentUnavailable: "Trimming is unavailable because the exact video duration could not be determined.",
	FragmentInputOutOfBoundsFmt:    "Range must stay within the video duration (%s).",
	FragmentURLStartOutOfBoundsFmt: "The URL timestamp is outside the video duration (%s).",

	PlVideosFmt: "  %d videos", PlEnterNums: "  Enter indices:", PlSelectedFmt: "  Selected: %d / %d",
	ErrPickOne: "Select at least one video",

	PlInputPlaceholder: "1-5, 2,4,7 or a (all)",
	PlParseEmpty:       "selection cannot be empty", PlParseRange: "range %d-%d outside 1–%d",
	PlParseNum: "number %d outside 1–%d", PlParseBad: "invalid token: %q", PlParseNone: "nothing selected",
	PlEmptyPlaylist: "playlist empty or unavailable", PlTimeout: "yt-dlp: timed out",
	VideoTitleFmt: "Video %d",

	ParallelFmt: "  Parallel downloads", QualityTitle: "  Choose quality", AudioTitle: "  Choose audio", WorkersQueuedFmt: "   %d items queued",

	ModeTitle: "  Choose mode", ModeVideo: "Video", ModeAudio: "Audio", ModeThumbnail: "Thumbnail",
	AudioMP3320: "Audio · MP3 320k", AudioMP3192: "Audio · MP3 192k", AudioM4ABest: "Audio · M4A/AAC Best",
	AudioOpusBest: "Audio · Opus Best", AudioFLAC: "Audio · FLAC Lossless", OutThumbnail: "Thumbnail",

	Downloading: "  Downloading…", PlaylistBarFmt: "  Playlist  ·  %d videos", QueueFmt: "◷ %d queued",
	Waiting: "waiting…", ErrSlot: "✘  download error", MergeProc: "Merging video+audio (ffmpeg)…", MP3Proc: "Converting audio (ffmpeg)…",
	ThumbProc: "Downloading thumbnail…",

	SummaryOK: "  ✔  Done!", SummaryFail: "  ✘  Download failed",
	SummaryPlaylistTitle: "Playlist finished", SessionHist: "  Session history:",
	SuccessFmt: "/%d ok",

	MenuUpdateY: "Yes, update", MenuUpdateN: "Skip",
	MenuVidOnly: "This video only", MenuOpenPl: "Open full playlist",
	MenuAgainY: "Download more", MenuAgainN: "Exit",
	WorkerSeq: "Sequential (1 worker)", WorkerNFmt: "%d workers",
	MenuFullVideo: "Full", MenuFromURLStart: "From URL timestamp", MenuManualRange: "Custom range",

	DepLabelFmt: "update %s",
	LangTab:     "Tab · language",

	FallbackFmt: "Fallback format #%d: %s",
	PlaylistTag: " [pl/%d]",
}

var strRU = UIStrings{
	HeaderPowered: "v%s   powered by yt-dlp",

	HelpUp: "вверх", HelpDown: "вниз", HelpEnter: "выбрать", HelpQuit: "выход",
	HelpDeps: "обновить зависимости", HelpSpace: "пробел", HelpAll: "все", HelpSlash: "номера",
	HelpSearch: "поиск", HelpBack: "назад",

	QBest: "▲ Лучшее качество (HD·4K)", QEcon: "▼ Экономичное (360p)",

	SpinnerUpdate: "  Проверяю обновления…", SpinnerPlaylist: "  Загружаю плейлист…", SpinnerQuality: "  Сканирую доступные качества…", SpinnerSearch: "  Ищу на YouTube…", SpinnerFragment: "  Определяю длительность видео…",

	UpdateAvail: "  ✔  Доступна версия ", CurrentVerShort: "  (сейчас %s)",
	PlaylistMixWarn: "  ⚠️ В ссылке и видео, и плейлист",

	DepsUpdating:      "Обновление зависимостей…",
	DepsRefreshing:    "Обновляю статус зависимостей…",
	UpdateAppliedWin:  "  Приложение сейчас закроется. Update bat заменит exe в фоне и удалится сам.",
	UpdateAppliedUnix: "  Бинарник заменён. Перезапустите программу.",
	UpdateDonePrefix:  "  ✔  Обновление применено — ",
	HelpAnyKey:        "любая клавиша", HelpExit: "выйти", DepsOK: "  ✔  Зависимости обновлены", ErrPrefix: "  ✘  Ошибка: ",

	AppSubtitle: "  ·  Загрузчик видео / аудио",
	TopBarDeps:  "↻ обновить зависимости", TopBarDepsBusy: " обновление…",

	PasteURL: "  Вставь ссылку на видео или плейлист", URLErrEmpty: "Ссылка не может быть пустой",
	URLErrBad: "Не похоже на YouTube-ссылку", URLHints: "\nyoutube.com/watch   youtube.com/playlist   youtu.be/",
	SearchTitle: "  Поиск на YouTube", SearchPrompt: "  Введи название видео или ключевые слова",
	SearchPlaceholder: "lofi hip hop mix", SearchErrEmpty: "Поисковый запрос не может быть пустым",
	SearchErrFailed: "Не удалось выполнить поиск", SearchNoResults: "Ничего не найдено",
	FragmentTitle: "  Фрагмент", FragmentHint: "  Опционально для одиночной загрузки в режимах видео и аудио",
	FragmentFromURLFmt: "С таймкода из URL (%s) до конца", FragmentInputTitle: "  Ввод диапазона фрагмента",
	FragmentInputPrompt: "  Формат: mm:ss-hh:mm:ss или hh:mm:ss-hh:mm:ss",
	FragmentInputHint:   "Пример: 1:00-2:30", FragmentInputHintWithDurationFmt: "Пример: 1:00-2:30\nДлительность видео: %s\nКонец диапазона не должен выходить за длительность видео.",
	FragmentInputBadFormat: "Неверный формат времени", FragmentInputBadRange: "Начало должно быть меньше конца",
	FragmentDurationFmt: "Длительность видео: %s", FragmentUnavailable: "Обрезка недоступна: не удалось определить точную длительность видео.",
	FragmentInputOutOfBoundsFmt:    "Диапазон должен укладываться в длительность видео (%s).",
	FragmentURLStartOutOfBoundsFmt: "Таймкод из URL выходит за длительность видео (%s).",

	PlVideosFmt: "  %d видео", PlEnterNums: "  Введи номера:", PlSelectedFmt: "  Выбрано: %d / %d",
	ErrPickOne: "Выбери хотя бы одно видео",

	PlInputPlaceholder: "1-5, 2,4,7 или а (все)",
	PlParseEmpty:       "выбор не может быть пустым", PlParseRange: "диапазон %d-%d вне 1–%d",
	PlParseNum: "номер %d вне 1–%d", PlParseBad: "непонятный ввод: %q", PlParseNone: "ничего не выбрано",
	PlEmptyPlaylist: "плейлист пуст или недоступен", PlTimeout: "yt-dlp: превышено время ожидания",
	VideoTitleFmt: "Видео %d",

	ParallelFmt: "  Параллельная загрузка", QualityTitle: "  Выбери качество", AudioTitle: "  Выбери аудио", WorkersQueuedFmt: "   %d видео в очереди",

	ModeTitle: "  Выбери режим", ModeVideo: "Видео", ModeAudio: "Аудио", ModeThumbnail: "Превью",
	AudioMP3320: "Аудио · MP3 320k", AudioMP3192: "Аудио · MP3 192k", AudioM4ABest: "Аудио · M4A/AAC Лучшее",
	AudioOpusBest: "Аудио · Opus Лучшее", AudioFLAC: "Аудио · FLAC Lossless", OutThumbnail: "Превью",

	Downloading: "  Загружаю…", PlaylistBarFmt: "  Плейлист  ·  %d видео", QueueFmt: "◷ %d в очереди",
	Waiting: "ожидание…", ErrSlot: "✘  ошибка загрузки", MergeProc: "слияние видео+аудио (ffmpeg)…", MP3Proc: "конвертация аудио (ffmpeg)…",
	ThumbProc: "скачивание превью…",

	SummaryOK: "  ✔  Готово!", SummaryFail: "  ✘  Не удалось скачать",
	SummaryPlaylistTitle: "Плейлист завершён", SessionHist: "  История сессии:",
	SuccessFmt: "/%d успешно",

	MenuUpdateY: "Да, обновить", MenuUpdateN: "Пропустить",
	MenuVidOnly: "Только это видео", MenuOpenPl: "Открыть плейлист",
	MenuAgainY: "Да, скачать ещё", MenuAgainN: "Нет, выйти",
	WorkerSeq: "Последовательно (1 поток)", WorkerNFmt: "%d потоков",
	MenuFullVideo: "Полностью", MenuFromURLStart: "С таймкода из URL", MenuManualRange: "Свой диапазон",

	DepLabelFmt: "обновление %s",
	LangTab:     "Tab · язык",

	FallbackFmt: "Запасной формат #%d: %s",
	PlaylistTag: " [плейлист/%d]",
}

var Loc = &strEN

func SyncLoc(l Locale) {
	if l == LocaleRU {
		Loc = &strRU
	} else {
		Loc = &strEN
	}
}

func StringsFor(l Locale) *UIStrings {
	if l == LocaleRU {
		return &strRU
	}
	return &strEN
}

func ActiveStrings() *UIStrings {
	return Loc
}

func PlaylistSuffix(l Locale, n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(StringsFor(l).PlaylistTag, n)
}
