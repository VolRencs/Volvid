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

var botWorkRoot = filepath.Join(app.DlDir, ".bot")

type Bot struct {
	api      *tg.Bot
	cfg      Config
	sessions *SessionStore
	maxSend  int64
	deps     app.CheckDepsResult

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

	opts := []tg.Option{
		tg.WithAllowedUpdates(tg.AllowedUpdates{
			models.AllowedUpdateMessage,
			models.AllowedUpdateCallbackQuery,
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

func (b *Bot) handleUpdate(update *models.Update) {
	switch {
	case update == nil:
		return
	case update.Message != nil:
		b.handleMessage(update.Message)
	case update.CallbackQuery != nil:
		b.handleCallback(update.CallbackQuery)
	}
}

func (b *Bot) apiCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), telegramAPITimeout)
}

func (b *Bot) fileSendCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), telegramFileSendTimeout)
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

func (b *Bot) send(chatID int64, text string) (models.Message, error) {
	ctx, cancel := b.apiCtx()
	defer cancel()

	msg, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:             chatID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: disabledPreview(),
	})
	if err != nil {
		return models.Message{}, err
	}
	return *msg, nil
}

func (b *Bot) sendKb(chatID int64, text string, kb models.InlineKeyboardMarkup) (models.Message, error) {
	ctx, cancel := b.apiCtx()
	defer cancel()

	msg, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:             chatID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: disabledPreview(),
		ReplyMarkup:        kb,
	})
	if err != nil {
		return models.Message{}, err
	}
	return *msg, nil
}

func (b *Bot) edit(chatID int64, msgID int, text string) {
	ctx, cancel := b.apiCtx()
	defer cancel()

	_, err := b.api.EditMessageText(ctx, &tg.EditMessageTextParams{
		ChatID:             chatID,
		MessageID:          msgID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: disabledPreview(),
	})
	if err != nil && !isMessageNotModified(err) {
		log.Printf("edit: %v", err)
	}
}

func (b *Bot) editKb(chatID int64, msgID int, text string, kb models.InlineKeyboardMarkup) {
	ctx, cancel := b.apiCtx()
	defer cancel()

	_, err := b.api.EditMessageText(ctx, &tg.EditMessageTextParams{
		ChatID:             chatID,
		MessageID:          msgID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: disabledPreview(),
		ReplyMarkup:        kb,
	})
	if err != nil && !isMessageNotModified(err) {
		log.Printf("editKb: %v", err)
	}
}

func (b *Bot) removeKb(chatID int64, msgID int) {
	ctx, cancel := b.apiCtx()
	defer cancel()

	_, err := b.api.EditMessageReplyMarkup(ctx, &tg.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   msgID,
		ReplyMarkup: emptyInlineKeyboard(),
	})
	if err != nil && !isMessageNotModified(err) {
		log.Printf("removeKb: %v", err)
	}
}

func (b *Bot) answer(cq *models.CallbackQuery, text string) {
	if cq == nil {
		return
	}

	ctx, cancel := b.apiCtx()
	defer cancel()

	params := &tg.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            text,
	}
	if text != "" {
		params.ShowAlert = true
	}
	if _, err := b.api.AnswerCallbackQuery(ctx, params); err != nil {
		log.Printf("answerCallback: %v", err)
	}
}

func (b *Bot) sendFile(chatID int64, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		log.Printf("sendFile stat %s: %v", path, err)
		return err
	}
	if fi.Size() > b.sendLimitBytes() {
		return errTelegramFileTooLarge
	}

	if b.cfg.LocalServer {
		err := b.sendFileByLocalPath(chatID, path)
		if err != nil {
			log.Printf("sendFile send %s: %v", path, err)
		}
		return err
	}

	err = b.sendFileByUpload(chatID, path)
	if err != nil {
		if isTelegramTooLarge(err) {
			return errTelegramFileTooLarge
		}
		log.Printf("sendFile send %s: %v", path, err)
		return err
	}
	return nil
}

func (b *Bot) sendFileByLocalPath(chatID int64, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Printf("sendFile abs %s: %v", path, err)
		return err
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
	return err
}

func (b *Bot) sendFileByUpload(chatID int64, path string) error {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("sendFile open %s: %v", path, err)
		return err
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
	return err
}

func localFileURI(path string) string {
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

const (
	cbPlVideo       = "pl:video"
	cbPlFull        = "pl:full"
	cbPlAll         = "pl:all"
	cbPlSelect      = "pl:select"
	cbPlTogglePref  = "pl:toggle:"
	cbPlPagePref    = "pl:page:"
	cbPlSelectAll   = "pl:sel:all"
	cbPlSelectNone  = "pl:sel:none"
	cbPlSelectDone  = "pl:sel:done"
	cbQualityPrefix = "q:"
	cbNoop          = "action:noop"
	cbCancel        = "action:cancel"
)

func kbPlaylistChoice() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				kbButton("🎬 Только это видео", cbPlVideo),
				kbButton("📋 Плейлист", cbPlFull),
			},
			{
				kbButton("❌ Отмена", cbCancel),
			},
		},
	}
}

func kbPlaylistScope() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				kbButton("⬇️ Весь плейлист", cbPlAll),
			},
			{
				kbButton("🎯 Выбрать видео", cbPlSelect),
			},
			{
				kbButton("❌ Отмена", cbCancel),
			},
		},
	}
}

func kbQuality(choices []app.QualityChoice) models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(choices)+1)
	for _, choice := range choices {
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton(choice.Label(app.LocaleRU), cbQualityPrefix+choice.Key),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		kbButton("❌ Отмена", cbCancel),
	})
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func emptyInlineKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}}
}

func kbButton(text, data string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{
		Text:         text,
		CallbackData: data,
	}
}

func isMessageNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "message is not modified")
}

func isTelegramTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "413") || strings.Contains(msg, "request entity too large")
}

func createBotWorkDir(chatID int64) (string, error) {
	if err := os.MkdirAll(botWorkRoot, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(botWorkRoot, fmt.Sprintf("%d-", chatID))
}
