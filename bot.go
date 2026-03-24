//go:build bot

package main

import (
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

func main() {
	logoutCloud := flag.Bool("logout-cloud", false, "one-time logout from cloud Bot API and exit")
	closeServer := flag.Bool("close-server", false, "close bot instance on the selected Bot API server and exit")
	deleteWebhook := flag.Bool("delete-webhook", false, "delete webhook on the selected Bot API server before start")
	dropPending := flag.Bool("drop-pending", false, "drop pending updates when deleting webhook")
	flag.Parse()

	token := strings.TrimSpace(BotToken)
	if token == "" {
		fmt.Fprintln(os.Stderr, "BotToken не задан в bot.go")
		os.Exit(1)
	}

	cfg := bot.Config{
		ServerURL:   strings.TrimSpace(BotAPIURL),
		LocalServer: BotUseLocalServer,
	}

	var err error
	if cfg.AdminIDs, err = parseIDSet(BotAdminIDs); err != nil {
		log.Fatalf("admins: %v", err)
	}
	if cfg.OwnerIDs, err = parseIDSet(BotOwnerIDs); err != nil {
		log.Fatalf("owners: %v", err)
	}

	switch {
	case *logoutCloud:
		if err := bot.LogoutCloud(token); err != nil {
			log.Fatalf("logout cloud: %v", err)
		}
		log.Println("Cloud Bot API: logout выполнен.")
		return
	case *closeServer:
		if err := bot.CloseServer(token, cfg); err != nil {
			log.Fatalf("close server: %v", err)
		}
		log.Println("Экземпляр бота закрыт на выбранном Bot API server.")
		return
	}

	if *deleteWebhook {
		if err := bot.DeleteWebhook(token, cfg, *dropPending); err != nil {
			log.Fatalf("delete webhook: %v", err)
		}
		log.Println("Webhook удалён на выбранном Bot API server.")
	}

	b, err := bot.NewWithConfig(token, cfg)
	if err != nil {
		log.Fatalf("запуск бота: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		b.Stop()
	}()

	log.Println("Бот запущен. Ctrl+C для остановки.")
	b.Run()
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
