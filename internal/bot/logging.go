package bot

import (
	"fmt"
	"log"
	"strings"
)

func (b *Bot) logf(format string, args ...any) {
	log.Printf("bot: "+format, args...)
}

func (b *Bot) logError(scope string, err error) {
	if err == nil {
		return
	}
	b.logf("%s: %v", scope, err)
}

func logChatUser(chatID, userID int64) string {
	return fmt.Sprintf("chat=%d user=%d", chatID, userID)
}

func logSnippet(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.Join(strings.Fields(text), " ")
	if limit <= 0 {
		return text
	}

	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}
