package app

import "fmt"

type UIStrings struct {
	HeaderPowered string

	HelpUp, HelpDown, HelpEnter, HelpQuit, HelpDeps, HelpSpace, HelpAll, HelpSlash string

	QBest, QEcon, QMP3 string

	SpinnerUpdate, SpinnerPlaylist, SpinnerQuality string

	UpdateAvail, CurrentVerShort string
	FFmpegWarn, FFmpegHint       string
	PlaylistMixWarn              string

	DepUpdatingVer, DepsUpdating                          string
	UpdateAppliedWin, UpdateAppliedUnix, UpdateDonePrefix string
	HelpAnyKey, HelpExit, DepsOK, ErrPrefix               string

	AppSubtitle, TopBarDeps, TopBarDepsBusy string

	PasteURL, URLErrEmpty, URLErrBad, URLHints string

	PlVideosFmt, PlEnterNums, PlSelectedFmt string
	ErrPickOne                              string

	PlInputPlaceholder                                              string
	PlParseEmpty, PlParseRange, PlParseNum, PlParseBad, PlParseNone string
	PlEmptyPlaylist, PlTimeout                                      string
	VideoTitleFmt                                                   string

	ParallelFmt, QualityTitle, WorkersQueuedFmt string

	Downloading, PlaylistBarFmt, QueueFmt, Waiting, ErrSlot, MergeProc, MP3Proc string

	SummaryOK, SummaryFail, SummaryPlaylistTitle, SessionHist, SuccessFmt string

	MenuUpdateY, MenuUpdateN, MenuFFmpegY, MenuFFmpegN, MenuVidOnly, MenuOpenPl string
	MenuAgainY, MenuAgainN, WorkerSeq, WorkerNFmt                               string

	DepLabelFmt string
	LangTab     string

	FallbackFmt string
	PlaylistTag string
}

var strEN = UIStrings{
	HeaderPowered: "v%s   powered by yt-dlp",

	HelpUp: "up", HelpDown: "down", HelpEnter: "enter", HelpQuit: "quit",
	HelpDeps: "update deps", HelpSpace: "space", HelpAll: "all", HelpSlash: "numbers",

	QBest: "▲ Best (HD·4K)", QEcon: "▼ Economy (360p)", QMP3: "♪ Audio only (MP3)",

	SpinnerUpdate: "  Checking for updates…", SpinnerPlaylist: "  Loading playlist…", SpinnerQuality: "  Scanning qualities…",

	UpdateAvail: "  ✔  New version ", CurrentVerShort: "  (current %s)",
	FFmpegWarn: "  ⚠️ ffmpeg not found", FFmpegHint: "     Required for HD, 4K and MP3",
	PlaylistMixWarn: "  ⚠️ URL has both a video and a playlist",

	DepUpdatingVer: "update ", DepsUpdating: "Updating dependencies…",
	UpdateAppliedWin:  "  The file will be replaced after exit. Run the new build manually.",
	UpdateAppliedUnix: "  Binary replaced. Restart the app.",
	UpdateDonePrefix:  "  ✔  Update applied — ",
	HelpAnyKey:        "any key", HelpExit: "exit", DepsOK: "  ✔  Dependencies updated", ErrPrefix: "  ✘  Error: ",

	AppSubtitle: "  ·  Video / Audio Downloader",
	TopBarDeps:  "↻ update dependencies", TopBarDepsBusy: " updating…",

	PasteURL: "  Paste a video or playlist URL", URLErrEmpty: "URL cannot be empty",
	URLErrBad: "Does not look like a YouTube URL", URLHints: "\nyoutube.com/watch   youtube.com/playlist   youtu.be/",

	PlVideosFmt: "  %d videos", PlEnterNums: "  Enter indices:", PlSelectedFmt: "  Selected: %d / %d",
	ErrPickOne: "Select at least one video",

	PlInputPlaceholder: "1-5, 2,4,7 or a (all)",
	PlParseEmpty:       "selection cannot be empty", PlParseRange: "range %d-%d outside 1–%d",
	PlParseNum: "number %d outside 1–%d", PlParseBad: "invalid token: %q", PlParseNone: "nothing selected",
	PlEmptyPlaylist: "playlist empty or unavailable", PlTimeout: "yt-dlp: timed out",
	VideoTitleFmt: "Video %d",

	ParallelFmt: "  Parallel downloads", QualityTitle: "  Choose quality", WorkersQueuedFmt: "   %d items queued",

	Downloading: "  Downloading…", PlaylistBarFmt: "  Playlist  ·  %d videos", QueueFmt: "◷ %d queued",
	Waiting: "waiting…", ErrSlot: "✘  download error", MergeProc: "Merging video+audio (ffmpeg)…", MP3Proc: "Converting to MP3 (ffmpeg)…",

	SummaryOK: "  ✔  Done!", SummaryFail: "  ✘  Download failed",
	SummaryPlaylistTitle: "Playlist finished", SessionHist: "  Session history:",
	SuccessFmt: "/%d ok",

	MenuUpdateY: "Yes, update", MenuUpdateN: "Skip",
	MenuFFmpegY: "Yes, download (~80 MB)", MenuFFmpegN: "Skip",
	MenuVidOnly: "This video only", MenuOpenPl: "Open full playlist",
	MenuAgainY: "Download more", MenuAgainN: "Exit",
	WorkerSeq: "Sequential (1 worker)", WorkerNFmt: "%d workers",

	DepLabelFmt: "update %s",
	LangTab:     "Tab · language",

	FallbackFmt: "Fallback format #%d: %s",
	PlaylistTag: " [pl/%d]",
}

var strRU = UIStrings{
	HeaderPowered: "v%s   powered by yt-dlp",

	HelpUp: "вверх", HelpDown: "вниз", HelpEnter: "выбрать", HelpQuit: "выход",
	HelpDeps: "обновить зависимости", HelpSpace: "пробел", HelpAll: "все", HelpSlash: "номера",

	QBest: "▲ Лучшее качество (HD·4K)", QEcon: "▼ Экономичное (360p)", QMP3: "♪ Только аудио (MP3)",

	SpinnerUpdate: "  Проверяю обновления…", SpinnerPlaylist: "  Загружаю плейлист…", SpinnerQuality: "  Сканирую доступные качества…",

	UpdateAvail: "  ✔  Доступна версия ", CurrentVerShort: "  (сейчас %s)",
	FFmpegWarn: "  ⚠️ ffmpeg не найден", FFmpegHint: "     Нужен для HD, 4K и MP3",
	PlaylistMixWarn: "  ⚠️ В ссылке и видео, и плейлист",

	DepUpdatingVer: "обновление ", DepsUpdating: "Обновление зависимостей…",
	UpdateAppliedWin:  "  Файл заменится после закрытия. Запустите вручную.",
	UpdateAppliedUnix: "  Бинарник заменён. Перезапустите программу.",
	UpdateDonePrefix:  "  ✔  Обновление применено — ",
	HelpAnyKey:        "любая клавиша", HelpExit: "выйти", DepsOK: "  ✔  Зависимости обновлены", ErrPrefix: "  ✘  Ошибка: ",

	AppSubtitle: "  ·  Загрузчик видео / аудио",
	TopBarDeps:  "↻ обновить зависимости", TopBarDepsBusy: " обновление…",

	PasteURL: "  Вставь ссылку на видео или плейлист", URLErrEmpty: "Ссылка не может быть пустой",
	URLErrBad: "Не похоже на YouTube-ссылку", URLHints: "\nyoutube.com/watch   youtube.com/playlist   youtu.be/",

	PlVideosFmt: "  %d видео", PlEnterNums: "  Введи номера:", PlSelectedFmt: "  Выбрано: %d / %d",
	ErrPickOne: "Выбери хотя бы одно видео",

	PlInputPlaceholder: "1-5, 2,4,7 или а (все)",
	PlParseEmpty:       "выбор не может быть пустым", PlParseRange: "диапазон %d-%d вне 1–%d",
	PlParseNum: "номер %d вне 1–%d", PlParseBad: "непонятный ввод: %q", PlParseNone: "ничего не выбрано",
	PlEmptyPlaylist: "плейлист пуст или недоступен", PlTimeout: "yt-dlp: превышено время ожидания",
	VideoTitleFmt: "Видео %d",

	ParallelFmt: "  Параллельная загрузка", QualityTitle: "  Выбери качество", WorkersQueuedFmt: "   %d видео в очереди",

	Downloading: "  Загружаю…", PlaylistBarFmt: "  Плейлист  ·  %d видео", QueueFmt: "◷ %d в очереди",
	Waiting: "ожидание…", ErrSlot: "✘  ошибка загрузки", MergeProc: "слияние видео+аудио (ffmpeg)…", MP3Proc: "конвертация в MP3 (ffmpeg)…",

	SummaryOK: "  ✔  Готово!", SummaryFail: "  ✘  Не удалось скачать",
	SummaryPlaylistTitle: "Плейлист завершён", SessionHist: "  История сессии:",
	SuccessFmt: "/%d успешно",

	MenuUpdateY: "Да, обновить", MenuUpdateN: "Пропустить",
	MenuFFmpegY: "Да, скачать (~80 МБ)", MenuFFmpegN: "Пропустить",
	MenuVidOnly: "Только это видео", MenuOpenPl: "Открыть плейлист",
	MenuAgainY: "Да, скачать ещё", MenuAgainN: "Нет, выйти",
	WorkerSeq: "Последовательно (1 поток)", WorkerNFmt: "%d потоков",

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
