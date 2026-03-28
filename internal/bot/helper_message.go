package bot

import (
	"strings"

	"github.com/go-telegram/bot/models"
)

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
