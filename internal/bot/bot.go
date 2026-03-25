package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	app "YouTubeBuild/internal/app"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var errTelegramFileTooLarge = errors.New("telegram file too large")
var errDownloadLimitExceeded = errors.New("download limit exceeded")

var botWorkRoot = filepath.Join(app.DlDir, ".bot")

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

	deps, err := app.EnsureRuntimeDeps(log.Printf)
	if err != nil {
		return nil, fmt.Errorf("подготовка зависимостей: %w", err)
	}

	b := &Bot{
		sessions: newSessionStore(),
		cfg:      cfg,
		maxSend:  cfg.maxSendBytes(),
		deps:     deps,
	}

	b.premium, err = newPremiumStore(cfg.PremiumUsersPath)
	if err != nil {
		return nil, fmt.Errorf("premium storage: %w", err)
	}
	b.users, err = newUserStore(cfg.KnownUsersPath)
	if err != nil {
		return nil, fmt.Errorf("users storage: %w", err)
	}
	b.timers, err = newTimerStore(cfg.TimersPath)
	if err != nil {
		return nil, fmt.Errorf("timers storage: %w", err)
	}
	b.scheduler = newScheduler(b.timers)
	b.scheduler.bind(b)

	opts := []tg.Option{
		tg.WithAllowedUpdates(tg.AllowedUpdates{
			models.AllowedUpdateMessage,
			models.AllowedUpdateCallbackQuery,
			"pre_checkout_query",
		}),
		tg.WithNotAsyncHandlers(),
		tg.WithDefaultHandler(func(_ context.Context, _ *tg.Bot, update *models.Update) {
			b.handleUpdate(update)
		}),
		tg.WithErrorsHandler(func(err error) {
			log.Printf("telegram api: %v", err)
		}),
	}
	if cfg.needsManualInitCheck() {
		opts = append(opts, tg.WithSkipGetMe())
	}
	opts = append(opts, cfg.options()...)

	api, err := tg.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("авторизация: %w", err)
	}
	if cfg.needsManualInitCheck() {
		if err := probeGetMe(token, cfg); err != nil {
			return nil, fmt.Errorf("авторизация: %w", err)
		}
	}
	b.api = api

	for _, dir := range []string{app.DepsDir, app.DlDir, botWorkRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("создание директории %s: %w", dir, err)
		}
	}

	log.Printf("Инициализирован Telegram бот id=%d backend=%q server=%s yt-dlp=%q ffmpeg=%q", api.ID(), cfg.backendLabel(), cfg.ServerURL, deps.YtdlpVer, deps.FFmpegVer)
	return b, nil
}

func (b *Bot) Run() {
	ctx, cancel := context.WithCancel(context.Background())

	b.mu.Lock()
	b.runCancel = cancel
	b.mu.Unlock()

	if b.scheduler != nil {
		b.scheduler.Start(ctx)
	}
	b.api.Start(ctx)
}

func (b *Bot) Stop() {
	b.mu.Lock()
	cancel := b.runCancel
	b.runCancel = nil
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (b *Bot) sendLimitBytes() int64 {
	if b == nil || b.maxSend <= 0 {
		return telegramCloudMaxSendBytes
	}
	return b.maxSend
}

func (b *Bot) sendLimitText() string {
	return app.FmtBytesFor(b.sendLimitBytes(), app.LocaleRU)
}

func (b *Bot) sendLimitNotice() string {
	return b.cfg.sendLimitNotice()
}

func (b *Bot) backendLabel() string {
	return b.cfg.backendLabel()
}

func (b *Bot) isOwnerID(id int64) bool {
	return b != nil && b.cfg.OwnerIDs.has(id)
}

func (b *Bot) isAdminID(id int64) bool {
	return b != nil && (b.cfg.AdminIDs.has(id) || b.cfg.OwnerIDs.has(id))
}

func (b *Bot) isOwnerMessage(msg *models.Message) bool {
	return b.isOwnerID(senderID(msg))
}

func (b *Bot) isAdminMessage(msg *models.Message) bool {
	return b.isAdminID(senderID(msg))
}

func disabledPreview() *models.LinkPreviewOptions {
	return &models.LinkPreviewOptions{IsDisabled: tg.True()}
}

func (b *Bot) sendFile(chatID int64, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if fi.Size() > b.sendLimitBytes() {
		return errTelegramFileTooLarge
	}

	var sendErr error
	if b.cfg.LocalServer {
		sendErr = b.sendFileByLocalPath(chatID, path)
	} else {
		sendErr = b.sendFileByUpload(chatID, path)
		if isTelegramTooLarge(sendErr) {
			return errTelegramFileTooLarge
		}
	}
	if sendErr != nil {
		log.Printf("sendFile %s: %v", path, sendErr)
	}
	return sendErr
}

func (b *Bot) sendFileByLocalPath(chatID int64, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs %s: %w", path, err)
	}

	ctx, cancel := b.fileSendCtx()
	defer cancel()

	_, err = b.api.SendDocument(ctx, &tg.SendDocumentParams{
		ChatID:   chatID,
		Document: &models.InputFileString{Data: localFileURI(absPath)},
	})
	if err != nil && isTelegramTooLarge(err) {
		return errTelegramFileTooLarge
	}
	if err != nil {
		return fmt.Errorf("send local file %s: %w", absPath, err)
	}
	return nil
}

func (b *Bot) sendFileByUpload(chatID int64, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	ctx, cancel := b.fileSendCtx()
	defer cancel()

	_, err = b.api.SendDocument(ctx, &tg.SendDocumentParams{
		ChatID: chatID,
		Document: &models.InputFileUpload{
			Filename: filepath.Base(path),
			Data:     f,
		},
	})
	if err != nil {
		return fmt.Errorf("upload file %s: %w", path, err)
	}
	return nil
}

func localFileURI(path string) string {
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func isTelegramTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "413") || strings.Contains(msg, "request entity too large")
}

func createBotWorkDir(userID int64) (string, error) {
	userRoot := filepath.Join(botWorkRoot, "users", fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(userRoot, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(userRoot, "job-")
}
