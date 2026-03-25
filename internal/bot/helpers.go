package bot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

func filesInDir(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out
}

func (b *Bot) sendDownloadedFiles(chatID int64, paths []string) (sent, tooLarge, sendErr int) {
	for _, path := range paths {
		err := b.sendFile(chatID, path)
		switch {
		case err == nil:
			sent++
		case errors.Is(err, errTelegramFileTooLarge):
			tooLarge++
		default:
			sendErr++
		}
	}
	return sent, tooLarge, sendErr
}

func (b *Bot) singleSendSummary(sent, tooLarge, sendErr int) string {
	switch {
	case sent > 0 && tooLarge == 0 && sendErr == 0:
		return "✅ Готово! Файл удалён с сервера."
	case sent > 0:
		return fmt.Sprintf(
			"✅ Скачано!\n\n📤 Отправлено: %d\n📦 Слишком большие для %s: %d\n⚠️ Не удалось отправить: %d\n\n%s\nФайлы удалены с сервера.",
			sent, b.backendLabel(), tooLarge, sendErr,
			b.sendLimitNotice(),
		)
	case tooLarge > 0:
		return fmt.Sprintf("✅ Скачано!\n\nФайл слишком большой для %s.\n\n%s\nФайл удалён с сервера.", b.backendLabel(), b.sendLimitNotice())
	default:
		return "✅ Скачано!\n\nНе удалось отправить файл в Telegram.\nФайл удалён с сервера."
	}
}

func (b *Bot) playlistSendSummary(icon string, done, failed, total, sent, tooLarge, sendErr int) string {
	var extra string
	switch {
	case sent > 0 || tooLarge > 0 || sendErr > 0:
		extra = fmt.Sprintf(
			"\n\n📤 Отправлено: %d\n📦 Слишком большие для %s: %d\n⚠️ Не удалось отправить: %d",
			sent, b.backendLabel(), tooLarge, sendErr,
		)
		if tooLarge > 0 {
			extra += "\n\n" + b.sendLimitNotice()
		}
		extra += "\nВсе файлы удалены с сервера."
	}

	return fmt.Sprintf(
		"%s <b>Плейлист завершён</b>\n\n✔ Успешно: %d\n✘ Ошибок: %d\nИтого: %d%s",
		icon, done, failed, total, extra,
	)
}

func cleanupBotWorkDir(dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	parent := filepath.Dir(dir)
	if parent != "" && parent != "." && parent != app.DlDir {
		_ = os.Remove(parent)
	}
}

func progressBar(done, total int) string {
	const width = 10
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := done * width / total
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}

func verLine(name, ver string) string {
	if ver == "" {
		return fmt.Sprintf("• %s — <b>не найден</b>", escapeHTML(name))
	}
	return fmt.Sprintf("• %s — <code>%s</code>", escapeHTML(name), escapeHTML(ver))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func messageIsCommand(msg *models.Message) bool {
	if msg == nil {
		return false
	}
	_, entities := commandTextAndEntities(msg)
	for _, entity := range entities {
		if entity.Type == models.MessageEntityTypeBotCommand && entity.Offset == 0 {
			return true
		}
	}
	return false
}

func messageCommand(msg *models.Message) string {
	if msg == nil {
		return ""
	}
	text, entities := commandTextAndEntities(msg)
	runes := []rune(text)
	for _, entity := range entities {
		if entity.Type != models.MessageEntityTypeBotCommand || entity.Offset != 0 || entity.Length <= 1 {
			continue
		}
		if entity.Offset+entity.Length > len(runes) {
			return ""
		}
		cmd := strings.TrimPrefix(string(runes[entity.Offset:entity.Offset+entity.Length]), "/")
		cmd, _, _ = strings.Cut(cmd, "@")
		return cmd
	}
	return ""
}

func messageCommandArgs(msg *models.Message) string {
	if msg == nil {
		return ""
	}
	text, entities := commandTextAndEntities(msg)
	cmdLen := 0
	for _, entity := range entities {
		if entity.Type == models.MessageEntityTypeBotCommand && entity.Offset == 0 {
			cmdLen = entity.Length
			break
		}
	}
	if cmdLen <= 0 {
		return ""
	}
	runes := []rune(text)
	if cmdLen >= len(runes) {
		return ""
	}
	return strings.TrimSpace(string(runes[cmdLen:]))
}

func commandTextAndEntities(msg *models.Message) (string, []models.MessageEntity) {
	if msg == nil {
		return "", nil
	}
	if strings.TrimSpace(msg.Text) != "" || len(msg.Entities) > 0 {
		return msg.Text, msg.Entities
	}
	return msg.Caption, msg.CaptionEntities
}

func callbackMessageMeta(cq *models.CallbackQuery) (chatID int64, msgID int, ok bool) {
	if cq == nil {
		return 0, 0, false
	}
	switch cq.Message.Type {
	case models.MaybeInaccessibleMessageTypeMessage:
		if cq.Message.Message == nil {
			return 0, 0, false
		}
		return cq.Message.Message.Chat.ID, cq.Message.Message.ID, true
	case models.MaybeInaccessibleMessageTypeInaccessibleMessage:
		if cq.Message.InaccessibleMessage == nil {
			return 0, 0, false
		}
		return cq.Message.InaccessibleMessage.Chat.ID, cq.Message.InaccessibleMessage.MessageID, true
	default:
		return 0, 0, false
	}
}

func isPrivateChatMessage(msg *models.Message) bool {
	return msg != nil && strings.EqualFold(string(msg.Chat.Type), "private")
}

func isForbiddenError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "403") || strings.Contains(msg, "bot was blocked") || strings.Contains(msg, "user is deactivated") || strings.Contains(msg, "chat not found")
}
