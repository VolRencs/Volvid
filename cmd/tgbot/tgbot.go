//go:build bot

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"YouTubeBuild/internal/bot"
)

func main() {
	token, cfg, err := loadBotBootstrapConfig()
	if err != nil {
		log.Fatalf("bot config: %v", err)
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

	log.Println("Бот запущен.")
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
