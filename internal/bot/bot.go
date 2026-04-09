package bot

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"sync"

	app "YouTubeBuild/internal/app"
	tg "github.com/go-telegram/bot"
)

var errTelegramFileTooLarge = errors.New("telegram file too large")
var errDownloadLimitExceeded = errors.New("download limit exceeded")

var botWorkRoot = filepath.Join(app.CacheDir, "bot", "jobs")

type Bot struct {
	api       *tg.Bot
	cfg       Config
	sessions  *SessionStore
	maxSend   int64
	deps      app.CheckDepsResult
	premium   *PremiumStore
	users     *UserStore
	timers    *TimerStore
	scheduler *Scheduler

	mu           sync.Mutex
	runCancel    context.CancelFunc
	depsUpdating bool
}

func New(token string) (*Bot, error) {
	return NewWithConfig(token, Config{})
}

func NewWithConfig(token string, cfg Config) (*Bot, error) {
	cfg = cfg.normalized()

	deps, err := ensureBotDependencies()
	if err != nil {
		return nil, err
	}

	b := &Bot{
		sessions: newSessionStore(),
		cfg:      cfg,
		maxSend:  cfg.maxSendBytes(),
		deps:     deps,
	}

	b.premium, b.users, b.timers, err = loadBotStores(cfg)
	if err != nil {
		return nil, err
	}
	b.scheduler = newScheduler(b.timers)
	b.scheduler.bind(b)

	api, err := newTelegramAPI(token, cfg, b.handleUpdate)
	if err != nil {
		return nil, err
	}
	b.api = api

	if err := ensureBotDirectories(); err != nil {
		return nil, err
	}

	log.Printf(
		"Инициализирован Telegram бот id=%d backend=%q server=%s yt-dlp=%q[%s] ffmpeg=%q[%s] node=%q[%s] cookies=%q js=%q",
		api.ID(),
		cfg.backendLabel(),
		cfg.ServerURL,
		deps.YTDLP.Version,
		deps.YTDLP.Source,
		deps.FFmpeg.Version,
		deps.FFmpeg.Source,
		deps.Node.Version,
		deps.Node.Source,
		cookiesDetail(deps.Cookies),
		runtimeDetail(deps.Runtime),
	)
	return b, nil
}
