package app

import "fmt"

type UIStrings struct {
	HelpMove, HelpEnter, HelpDeps, HelpSpace, HelpAll, HelpSlash, HelpSearch, HelpBack, HelpOpenFolder, HelpPickFolder string

	QBest, QEcon string

	SpinnerUpdate, SpinnerPlaylist, SpinnerQuality, SpinnerSearch, SpinnerFragment string

	UpdateAvail, CurrentVerShort string
	PlaylistMixWarn              string

	DepsUpdating, DepsRefreshing                          string
	UpdateAppliedWin, UpdateAppliedUnix, UpdateDonePrefix string
	HelpAnyKey, HelpExit, DepsOK                          string
	DownloadsDirLocked, PickDownloadsFailed               string

	PasteURL, URLErrEmpty, URLErrBad, URLHints, PickDownloadsTitle                                        string
	SearchTitle, SearchPrompt, SearchPlaceholder, SearchErrEmpty, SearchErrFailed, SearchNoResults        string
	FragmentTitle, FragmentHint, FragmentFromURLFmt, FragmentInputTitle, FragmentInputPrompt              string
	FragmentInputHint, FragmentInputHintWithDurationFmt, FragmentInputBadFormat, FragmentInputBadRange    string
	FragmentDurationFmt, FragmentUnavailable, FragmentInputOutOfBoundsFmt, FragmentURLStartOutOfBoundsFmt string

	HomeInputTitle, HomeOutputTitle, HomeActionsTitle, HomeRuntimeTitle, HomeSessionTitle, HomeOverviewTitle string
	HomeSessionEmpty, HomeStatSuccess, HomeStatFailed                                                        string

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

	SummaryOK, SummaryPartial, SummaryFail, SummaryPlaylistTitle, SummaryLocation, SessionHist, SuccessFmt string

	MenuUpdateY, MenuUpdateN, MenuVidOnly, MenuOpenPl string
	MenuAgainY, MenuAgainN, WorkerSeq, WorkerNFmt     string
	MenuFullVideo, MenuFromURLStart, MenuManualRange  string

	DepLabelFmt, DepTitle, DepSubtitle, DepSystemNote  string
	DepActionDownloadFmt, DepActionUpdateFmt           string
	DepActionRefresh, DepActionContinue                string
	DepActionBack, DepActionExit                       string
	DepRequirementFmt                                  string
	DepStatusActive, DepStatusMissing                  string
	DepStatusNotActive, DepStatusAvailable             string
	DepStatusChecking                                  string
	DepRoleRequired, DepRoleOptional                   string
	DepSourceBundled, DepSourceSystem                  string
	NoticeInfo, NoticeSuccess, NoticeWarn, NoticeError string
	LangTab                                            string

	FallbackFmt string
	PlaylistTag string
}

var strEN = UIStrings{
	HelpMove: "move", HelpEnter: "continue",
	HelpDeps: "dependencies", HelpSpace: "select", HelpAll: "all", HelpSlash: "manual",
	HelpSearch: "search", HelpBack: "back", HelpOpenFolder: "open folder", HelpPickFolder: "choose folder",

	QBest: "Best quality (HD·4K)", QEcon: "Economy quality (360p)",

	SpinnerUpdate: "Checking for updates…", SpinnerPlaylist: "Loading playlist…", SpinnerQuality: "Scanning available qualities…", SpinnerSearch: "Searching YouTube…", SpinnerFragment: "Detecting video duration…",

	UpdateAvail:     "New version available",
	CurrentVerShort: "current %s",
	PlaylistMixWarn: "URL contains both a video and a playlist.",

	DepsUpdating:      "Updating dependencies…",
	DepsRefreshing:    "Refreshing dependency status…",
	UpdateAppliedWin:  "The app will close and replace the executable in the background.",
	UpdateAppliedUnix: "Binary replaced. Restart the app.",
	UpdateDonePrefix:  "Update applied",
	HelpAnyKey:        "Any key", HelpExit: "exit", DepsOK: "Dependencies updated.",
	DownloadsDirLocked:  "Download location is fixed by VOLREN_DOWNLOADS_DIR.",
	PickDownloadsFailed: "Failed to choose download folder",

	PasteURL: "Paste a YouTube video or playlist URL", URLErrEmpty: "URL cannot be empty",
	URLErrBad: "Does not look like a YouTube URL", URLHints: "youtube.com/watch · youtube.com/playlist · youtu.be",
	PickDownloadsTitle: "Choose download folder",
	HomeInputTitle:     "Target", HomeOutputTitle: "Download location", HomeActionsTitle: "Quick actions",
	HomeRuntimeTitle: "Environment", HomeSessionTitle: "Recent session", HomeSessionEmpty: "No downloads yet in this session.",
	HomeOverviewTitle: "Overview",
	HomeStatSuccess:   "ok", HomeStatFailed: "failed",
	SearchTitle: "YouTube search", SearchPrompt: "Enter a video title or keywords",
	SearchPlaceholder: "lofi hip hop mix", SearchErrEmpty: "Search query cannot be empty",
	SearchErrFailed: "Search failed", SearchNoResults: "Nothing found",
	FragmentTitle: "Fragment", FragmentHint: "Optional for single video or audio downloads.",
	FragmentFromURLFmt: "From URL timestamp (%s) to end", FragmentInputTitle: "Enter fragment range",
	FragmentInputPrompt: "Format: mm:ss-mm:ss or hh:mm:ss-hh:mm:ss",
	FragmentInputHint:   "Example: 1:00-2:30", FragmentInputHintWithDurationFmt: "Example: 1:00-2:30\nVideo duration: %s\nEnd cannot exceed the video duration.",
	FragmentInputBadFormat: "Invalid time format", FragmentInputBadRange: "Start time must be less than end time",
	FragmentDurationFmt: "Video duration: %s", FragmentUnavailable: "Trimming is unavailable because the exact video duration could not be determined.",
	FragmentInputOutOfBoundsFmt:    "Range must stay within the video duration (%s).",
	FragmentURLStartOutOfBoundsFmt: "The URL timestamp is outside the video duration (%s).",

	PlVideosFmt: "%d videos", PlEnterNums: "Enter indices", PlSelectedFmt: "Selected %d / %d",
	ErrPickOne: "Select at least one video",

	PlInputPlaceholder: "1-5, 2,4,7 or a (all)",
	PlParseEmpty:       "selection cannot be empty", PlParseRange: "range %d-%d outside 1–%d",
	PlParseNum: "number %d outside 1–%d", PlParseBad: "invalid token: %q", PlParseNone: "nothing selected",
	PlEmptyPlaylist: "playlist empty or unavailable", PlTimeout: "yt-dlp: timed out",
	VideoTitleFmt: "Video %d",

	ParallelFmt: "Parallel downloads", QualityTitle: "Choose quality", AudioTitle: "Choose audio", WorkersQueuedFmt: "%d items queued",

	ModeTitle: "Choose mode", ModeVideo: "Video", ModeAudio: "Audio", ModeThumbnail: "Thumbnail",
	AudioMP3320: "Audio · MP3 320k", AudioMP3192: "Audio · MP3 192k", AudioM4ABest: "Audio · M4A/AAC Best",
	AudioOpusBest: "Audio · Opus Best", AudioFLAC: "Audio · FLAC Lossless", OutThumbnail: "Thumbnail",

	Downloading: "Downloading", PlaylistBarFmt: "Playlist · %d items", QueueFmt: "%d queued",
	Waiting: "Waiting for work", ErrSlot: "Download failed", MergeProc: "Merging video and audio", MP3Proc: "Converting audio",
	ThumbProc: "Downloading thumbnail",

	SummaryOK: "Download complete", SummaryPartial: "Completed with issues", SummaryFail: "Download failed",
	SummaryPlaylistTitle: "Playlist summary", SummaryLocation: "Saved to", SessionHist: "Session history",
	SuccessFmt: "/%d ok",

	MenuUpdateY: "Install update", MenuUpdateN: "Skip for now",
	MenuVidOnly: "This video only", MenuOpenPl: "Open full playlist",
	MenuAgainY: "Download more", MenuAgainN: "Exit",
	WorkerSeq: "Sequential · 1 worker", WorkerNFmt: "%d workers",
	MenuFullVideo: "Full media", MenuFromURLStart: "From URL timestamp", MenuManualRange: "Custom range",

	DepLabelFmt:          "Updating %s",
	DepTitle:             "Dependencies",
	DepSubtitle:          "yt-dlp · ffmpeg · node · cookies · js runtime",
	DepSystemNote:        "System dependencies are not updated here.",
	DepActionDownloadFmt: "Download %s",
	DepActionUpdateFmt:   "Update %s",
	DepActionRefresh:     "Refresh status",
	DepActionContinue:    "Continue",
	DepActionBack:        "Back",
	DepActionExit:        "Exit",
	DepRequirementFmt:    "%s is required for this mode.",
	DepStatusActive:      "active",
	DepStatusMissing:     "missing",
	DepStatusNotActive:   "not active",
	DepStatusAvailable:   "available",
	DepStatusChecking:    "checking...",
	DepRoleRequired:      "required",
	DepRoleOptional:      "optional",
	DepSourceBundled:     "bundled",
	DepSourceSystem:      "system",
	NoticeInfo:           "INFO",
	NoticeSuccess:        "OK",
	NoticeWarn:           "WARN",
	NoticeError:          "ERROR",
	LangTab:              "Tab language",

	FallbackFmt: "Fallback format #%d: %s",
	PlaylistTag: " [pl/%d]",
}

var strRU = UIStrings{
	HelpMove: "движение", HelpEnter: "продолжить",
	HelpDeps: "зависимости", HelpSpace: "выбрать", HelpAll: "все", HelpSlash: "вручную",
	HelpSearch: "поиск", HelpBack: "назад", HelpOpenFolder: "открыть папку", HelpPickFolder: "выбрать папку",

	QBest: "Лучшее качество (HD·4K)", QEcon: "Экономичное качество (360p)",

	SpinnerUpdate: "Проверяю обновления…", SpinnerPlaylist: "Загружаю плейлист…", SpinnerQuality: "Сканирую доступные качества…", SpinnerSearch: "Ищу на YouTube…", SpinnerFragment: "Определяю длительность видео…",

	UpdateAvail:     "Доступно обновление",
	CurrentVerShort: "текущая %s",
	PlaylistMixWarn: "В ссылке есть и видео, и плейлист.",

	DepsUpdating:      "Обновление зависимостей…",
	DepsRefreshing:    "Обновляю статус зависимостей…",
	UpdateAppliedWin:  "Приложение закроется и заменит исполняемый файл в фоне.",
	UpdateAppliedUnix: "Бинарный файл обновлён. Перезапустите приложение.",
	UpdateDonePrefix:  "Обновление применено",
	HelpAnyKey:        "Любая клавиша", HelpExit: "выйти", DepsOK: "Зависимости обновлены.",
	DownloadsDirLocked:  "Папка загрузки зафиксирована через VOLREN_DOWNLOADS_DIR.",
	PickDownloadsFailed: "Не удалось выбрать папку загрузки",

	PasteURL: "Вставь ссылку на видео или плейлист YouTube", URLErrEmpty: "Ссылка не может быть пустой",
	URLErrBad: "Не похоже на YouTube-ссылку", URLHints: "youtube.com/watch · youtube.com/playlist · youtu.be",
	PickDownloadsTitle: "Выбери папку загрузки",
	HomeInputTitle:     "Источник", HomeOutputTitle: "Папка загрузки", HomeActionsTitle: "Быстрые действия",
	HomeRuntimeTitle: "Окружение", HomeSessionTitle: "Текущая сессия", HomeSessionEmpty: "В этой сессии ещё не было загрузок.",
	HomeOverviewTitle: "Сводка",
	HomeStatSuccess:   "успешно", HomeStatFailed: "ошибки",
	SearchTitle: "Поиск YouTube", SearchPrompt: "Введи название видео или ключевые слова",
	SearchPlaceholder: "lofi hip hop mix", SearchErrEmpty: "Поисковый запрос не может быть пустым",
	SearchErrFailed: "Не удалось выполнить поиск", SearchNoResults: "Ничего не найдено",
	FragmentTitle: "Фрагмент", FragmentHint: "Опционально для одиночной загрузки видео или аудио.",
	FragmentFromURLFmt: "С таймкода из URL (%s) до конца", FragmentInputTitle: "Ввод диапазона фрагмента",
	FragmentInputPrompt: "Формат: mm:ss-mm:ss или hh:mm:ss-hh:mm:ss",
	FragmentInputHint:   "Пример: 1:00-2:30", FragmentInputHintWithDurationFmt: "Пример: 1:00-2:30\nДлительность видео: %s\nКонец диапазона не должен выходить за длительность видео.",
	FragmentInputBadFormat: "Неверный формат времени", FragmentInputBadRange: "Начало должно быть меньше конца",
	FragmentDurationFmt: "Длительность видео: %s", FragmentUnavailable: "Обрезка недоступна: не удалось определить точную длительность видео.",
	FragmentInputOutOfBoundsFmt:    "Диапазон должен укладываться в длительность видео (%s).",
	FragmentURLStartOutOfBoundsFmt: "Таймкод из URL выходит за длительность видео (%s).",

	PlVideosFmt: "%d видео", PlEnterNums: "Введи номера", PlSelectedFmt: "Выбрано %d / %d",
	ErrPickOne: "Выбери хотя бы одно видео",

	PlInputPlaceholder: "1-5, 2,4,7 или а (все)",
	PlParseEmpty:       "выбор не может быть пустым", PlParseRange: "диапазон %d-%d вне 1–%d",
	PlParseNum: "номер %d вне 1–%d", PlParseBad: "непонятный ввод: %q", PlParseNone: "ничего не выбрано",
	PlEmptyPlaylist: "плейлист пуст или недоступен", PlTimeout: "yt-dlp: превышено время ожидания",
	VideoTitleFmt: "Видео %d",

	ParallelFmt: "Параллельная загрузка", QualityTitle: "Выбери качество", AudioTitle: "Выбери аудио", WorkersQueuedFmt: "%d видео в очереди",

	ModeTitle: "Выбери режим", ModeVideo: "Видео", ModeAudio: "Аудио", ModeThumbnail: "Превью",
	AudioMP3320: "Аудио · MP3 320k", AudioMP3192: "Аудио · MP3 192k", AudioM4ABest: "Аудио · M4A/AAC Лучшее",
	AudioOpusBest: "Аудио · Opus Лучшее", AudioFLAC: "Аудио · FLAC Lossless", OutThumbnail: "Превью",

	Downloading: "Загрузка", PlaylistBarFmt: "Плейлист · %d видео", QueueFmt: "%d в очереди",
	Waiting: "Ожидание очереди", ErrSlot: "Загрузка не удалась", MergeProc: "Объединение видео и аудио", MP3Proc: "Конвертация аудио",
	ThumbProc: "Загрузка превью",

	SummaryOK: "Загрузка завершена", SummaryPartial: "Завершено с ошибками", SummaryFail: "Загрузка не удалась",
	SummaryPlaylistTitle: "Итог по плейлисту", SummaryLocation: "Сохранено в", SessionHist: "История сессии",
	SuccessFmt: "/%d успешно",

	MenuUpdateY: "Установить обновление", MenuUpdateN: "Пропустить",
	MenuVidOnly: "Только это видео", MenuOpenPl: "Открыть плейлист",
	MenuAgainY: "Скачать ещё", MenuAgainN: "Выйти",
	WorkerSeq: "Последовательно · 1 поток", WorkerNFmt: "%d потоков",
	MenuFullVideo: "Полностью", MenuFromURLStart: "С таймкода из URL", MenuManualRange: "Свой диапазон",

	DepLabelFmt:          "Обновление %s",
	DepTitle:             "Зависимости",
	DepSubtitle:          "yt-dlp · ffmpeg · node · cookies · js runtime",
	DepSystemNote:        "Системные зависимости здесь не обновляются.",
	DepActionDownloadFmt: "Скачать %s",
	DepActionUpdateFmt:   "Обновить %s",
	DepActionRefresh:     "Обновить статус",
	DepActionContinue:    "Продолжить",
	DepActionBack:        "Назад",
	DepActionExit:        "Выход",
	DepRequirementFmt:    "%s требуется для этого режима.",
	DepStatusActive:      "активно",
	DepStatusMissing:     "не найден",
	DepStatusNotActive:   "не активно",
	DepStatusAvailable:   "доступно",
	DepStatusChecking:    "проверяю...",
	DepRoleRequired:      "обязательно",
	DepRoleOptional:      "опционально",
	DepSourceBundled:     "в комплекте",
	DepSourceSystem:      "система",
	NoticeInfo:           "ИНФО",
	NoticeSuccess:        "OK",
	NoticeWarn:           "ВНИМ",
	NoticeError:          "ОШИБКА",
	LangTab:              "Tab язык",

	FallbackFmt: "Запасной формат #%d: %s",
	PlaylistTag: " [плейлист/%d]",
}

func StringsFor(l Locale) *UIStrings {
	if l == LocaleRU {
		return &strRU
	}
	return &strEN
}

func PlaylistSuffix(l Locale, n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(StringsFor(l).PlaylistTag, n)
}
