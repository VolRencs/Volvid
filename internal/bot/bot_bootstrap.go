package bot

import (
	"context"
	"fmt"
	"log"
	"os"

	app "YouTubeBuild/internal/app"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func ensureBotDependencies() (app.CheckDepsResult, error) {
	deps, err := app.EnsureRuntimeDeps(log.Printf)
	if err != nil {
		return app.CheckDepsResult{}, fmt.Errorf("подготовка зависимостей: %w", err)
	}
	return deps, nil
}

func loadBotStores(cfg Config) (*PremiumStore, *UserStore, *TimerStore, error) {
	premium, err := newPremiumStore(cfg.PremiumUsersPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("premium storage: %w", err)
	}

	users, err := newUserStore(cfg.KnownUsersPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("users storage: %w", err)
	}

	timers, err := newTimerStore(cfg.TimersPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("timers storage: %w", err)
	}

	return premium, users, timers, nil
}

func newTelegramAPI(token string, cfg Config, updateHandler func(*models.Update)) (*tg.Bot, error) {
	opts := []tg.Option{
		tg.WithAllowedUpdates(tg.AllowedUpdates{
			models.AllowedUpdateMessage,
			models.AllowedUpdateCallbackQuery,
			"pre_checkout_query",
		}),
		tg.WithNotAsyncHandlers(),
		tg.WithDefaultHandler(func(_ context.Context, _ *tg.Bot, update *models.Update) {
			updateHandler(update)
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
	return api, nil
}

func ensureBotDirectories() error {
	for _, dir := range []string{app.DepsDir, app.DlDir, botWorkRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("создание директории %s: %w", dir, err)
		}
	}
	return nil
}
