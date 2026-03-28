package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) Run() {
	ctx, cancel := context.WithCancel(context.Background())

	b.mu.Lock()
	b.runCancel = cancel
	b.mu.Unlock()

	if b.scheduler != nil {
		b.logf("starting scheduler")
		b.scheduler.Start(ctx)
	}
	b.logf("starting polling loop")
	b.api.Start(ctx)
}

func (b *Bot) Stop() {
	b.mu.Lock()
	cancel := b.runCancel
	b.runCancel = nil
	b.mu.Unlock()

	if cancel != nil {
		b.logf("stop requested")
		cancel()
	}
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

func createBotWorkDir(userID int64) (string, error) {
	userRoot := filepath.Join(botWorkRoot, "users", fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(userRoot, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(userRoot, "job-")
}
