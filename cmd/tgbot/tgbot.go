//go:build bot

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"YouTubeBuild/internal/bot"
)

var (
	// Telegram bot token from BotFather.
	BotToken = ""

	// Local Bot API Server settings.
	BotUseLocalServer = true
	BotAPIURL         = "http://127.0.0.1:8081"

	// Comma-separated Telegram user IDs.
	BotAdminIDs = ""
	BotOwnerIDs = ""
)

type runtimeFlags struct {
	logoutCloud   bool
	closeServer   bool
	deleteWebhook bool
	dropPending   bool
}

func main() {
	flags := parseRuntimeFlags()
	token := strings.TrimSpace(BotToken)
	if token == "" {
		fmt.Fprintln(os.Stderr, "BotToken не задан в cmd/tgbot/tgbot.go")
		os.Exit(1)
	}

	cfg, err := loadBotConfig()
	if err != nil {
		log.Fatalf("bot config: %v", err)
	}

	switch {
	case flags.logoutCloud:
		if err := bot.LogoutCloud(token); err != nil {
			log.Fatalf("logout cloud: %v", err)
		}
		log.Println("Cloud Bot API: logout выполнен.")
		return
	case flags.closeServer:
		if err := bot.CloseServer(token, cfg); err != nil {
			log.Fatalf("close server: %v", err)
		}
		log.Println("Экземпляр бота закрыт на выбранном Bot API server.")
		return
	}

	if flags.deleteWebhook {
		if err := bot.DeleteWebhook(token, cfg, flags.dropPending); err != nil {
			log.Fatalf("delete webhook: %v", err)
		}
		log.Println("Webhook удалён на выбранном Bot API server.")
	}

	b, err := bot.NewWithConfig(token, cfg)
	if err != nil {
		log.Fatalf("запуск бота: %v", err)
	}

	stopCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	go func() {
		<-stopCtx.Done()
		b.Stop()
	}()

	log.Println("Бот запущен. Ctrl+C для остановки.")
	b.Run()
}

func parseRuntimeFlags() runtimeFlags {
	flags := runtimeFlags{}
	flag.BoolVar(&flags.logoutCloud, "logout-cloud", false, "one-time logout from cloud Bot API and exit")
	flag.BoolVar(&flags.closeServer, "close-server", false, "close bot instance on the selected Bot API server and exit")
	flag.BoolVar(&flags.deleteWebhook, "delete-webhook", false, "delete webhook on the selected Bot API server before start")
	flag.BoolVar(&flags.dropPending, "drop-pending", false, "drop pending updates when deleting webhook")
	flag.Parse()
	return flags
}

func loadBotConfig() (bot.Config, error) {
	cfg := bot.Config{
		ServerURL:   strings.TrimSpace(BotAPIURL),
		LocalServer: BotUseLocalServer,
	}

	var err error
	if cfg.AdminIDs, err = parseIDSet(BotAdminIDs); err != nil {
		return bot.Config{}, fmt.Errorf("admins: %w", err)
	}
	if cfg.OwnerIDs, err = parseIDSet(BotOwnerIDs); err != nil {
		return bot.Config{}, fmt.Errorf("owners: %w", err)
	}
	return cfg, nil
}

func parseIDSet(raw string) (map[int64]struct{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[int64]struct{}{}, nil
	}

	out := make(map[int64]struct{})
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	}) {
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("неверный Telegram ID %q", part)
		}
		out[id] = struct{}{}
	}
	return out, nil
}
