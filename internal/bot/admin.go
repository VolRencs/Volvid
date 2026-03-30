package bot

import (
	"fmt"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	broadcastTypeText = "text"
	broadcastTypeCopy = "copy"

	commandUnavailableText = "Команда недоступна."
	broadcastUsageText     = "Использование:\n/broadcast <текст>\nили reply /broadcast на сообщение с текстом/медиа."
	scheduleUsageText      = "Использование:\n/schedule 2h <текст>\nили reply /schedule 2h на сообщение."
	delTimerUsageText      = "Использование: /deltimer <id>"
)

type broadcastPayload struct {
	Type            string
	Text            string
	Caption         string
	SourceChatID    int64
	SourceMessageID int
}

func (b *Bot) handleBroadcastCommand(msg *models.Message) {
	chatID := msg.Chat.ID
	if !b.ensureAdminCommand(msg) {
		return
	}

	payload, err := broadcastPayloadFromMessage(msg)
	if err != nil {
		b.send(chatID, broadcastUsageText)
		return
	}

	b.logf("broadcast start %s type=%s audience=%d", logChatUser(chatID, senderID(msg)), payload.Type, len(b.users.List()))
	sent, failed := b.broadcastAll(payload)
	b.logf("broadcast done %s sent=%d failed=%d", logChatUser(chatID, senderID(msg)), sent, failed)
	b.send(chatID, fmt.Sprintf("<b>Рассылка завершена.</b>\n\nОтправлено: %d\nОшибок: %d", sent, failed))
}

func (b *Bot) handleScheduleCommand(msg *models.Message) {
	chatID := msg.Chat.ID
	if !b.ensureAdminCommand(msg) {
		return
	}

	args := strings.Fields(messageCommandArgs(msg))
	if len(args) == 0 {
		b.send(chatID, scheduleUsageText)
		return
	}

	interval, err := time.ParseDuration(args[0])
	if err != nil || interval <= 0 {
		b.send(chatID, "Неверный интервал. Пример: 30m, 2h, 1h30m")
		return
	}

	payload, err := schedulePayloadFromMessage(msg, strings.TrimSpace(strings.TrimPrefix(messageCommandArgs(msg), args[0])))
	if err != nil {
		b.send(chatID, scheduleUsageText)
		return
	}

	entry := TimerEntry{
		ID:              fmt.Sprintf("tmr_%d", time.Now().UnixNano()),
		Interval:        interval.String(),
		NextRun:         time.Now().Add(interval),
		Type:            payload.Type,
		Text:            payload.Text,
		Caption:         payload.Caption,
		SourceChatID:    payload.SourceChatID,
		SourceMessageID: payload.SourceMessageID,
	}
	if err := b.scheduler.Add(entry); err != nil {
		b.logError(fmt.Sprintf("schedule add %s id=%s", logChatUser(chatID, senderID(msg)), entry.ID), err)
		b.send(chatID, "Не удалось создать таймер.")
		return
	}
	b.logf("timer created %s id=%s interval=%s type=%s", logChatUser(chatID, senderID(msg)), entry.ID, entry.Interval, entry.Type)

	b.send(chatID, fmt.Sprintf("<b>Таймер создан.</b>\n\nID: <code>%s</code>\nИнтервал: <code>%s</code>\nСледующий запуск: <code>%s</code>", entry.ID, entry.Interval, entry.NextRun.Format(time.RFC3339)))
}

func (b *Bot) handleTimersCommand(msg *models.Message) {
	if !b.ensureAdminCommand(msg) {
		return
	}

	items := b.scheduler.List()
	if len(items) == 0 {
		b.send(msg.Chat.ID, "Активных таймеров нет.")
		return
	}

	lines := []string{"<b>Активные таймеры</b>", ""}
	for _, entry := range items {
		lines = append(lines, fmt.Sprintf("• <code>%s</code> · %s · %s · %s", entry.ID, escapeHTML(entry.Interval), escapeHTML(entry.Type), escapeHTML(entry.NextRun.Format(time.RFC3339))))
	}
	b.send(msg.Chat.ID, strings.Join(lines, "\n"))
}

func (b *Bot) handleDelTimerCommand(msg *models.Message) {
	if !b.ensureAdminCommand(msg) {
		return
	}

	id := strings.TrimSpace(messageCommandArgs(msg))
	if id == "" {
		b.send(msg.Chat.ID, delTimerUsageText)
		return
	}
	if err := b.scheduler.Remove(id); err != nil {
		b.logError(fmt.Sprintf("timer delete %s id=%s", logChatUser(msg.Chat.ID, senderID(msg)), id), err)
		b.send(msg.Chat.ID, "Не удалось удалить таймер.")
		return
	}
	b.logf("timer deleted %s id=%s", logChatUser(msg.Chat.ID, senderID(msg)), id)
	b.send(msg.Chat.ID, "Таймер удалён.")
}

func (b *Bot) executeTimer(entry TimerEntry) error {
	payload := broadcastPayload{
		Type:            entry.Type,
		Text:            entry.Text,
		Caption:         entry.Caption,
		SourceChatID:    entry.SourceChatID,
		SourceMessageID: entry.SourceMessageID,
	}
	_, failed := b.broadcastAll(payload)
	if failed > 0 {
		return fmt.Errorf("failed deliveries: %d", failed)
	}
	return nil
}

func (b *Bot) broadcastAll(payload broadcastPayload) (sent, failed int) {
	for _, userID := range b.users.List() {
		if err := b.sendBroadcast(userID, payload); err != nil {
			failed++
			b.logError(fmt.Sprintf("broadcast delivery user=%d type=%s", userID, payload.Type), err)
			if isForbiddenError(err) {
				_ = b.users.Remove(userID)
				b.logf("broadcast delivery removed user=%d after forbidden error", userID)
			}
			continue
		}
		sent++
	}
	return sent, failed
}

func (b *Bot) sendBroadcast(chatID int64, payload broadcastPayload) error {
	switch payload.Type {
	case broadcastTypeCopy:
		return b.copyMessage(chatID, payload)
	default:
		_, err := b.send(chatID, escapeHTML(payload.Text))
		return err
	}
}

func (b *Bot) copyMessage(chatID int64, payload broadcastPayload) error {
	ctx, cancel := b.apiCtx()
	defer cancel()

	params := &tg.CopyMessageParams{
		ChatID:     chatID,
		FromChatID: payload.SourceChatID,
		MessageID:  payload.SourceMessageID,
	}
	if strings.TrimSpace(payload.Caption) != "" {
		params.Caption = payload.Caption
	}
	_, err := b.api.CopyMessage(ctx, params)
	if err != nil {
		b.logError(fmt.Sprintf("copyMessage chat=%d from_chat=%d msg=%d", chatID, payload.SourceChatID, payload.SourceMessageID), err)
	}
	return err
}

func broadcastPayloadFromMessage(msg *models.Message) (broadcastPayload, error) {
	return payloadFromMessage(msg, messageCommandArgs(msg))
}

func schedulePayloadFromMessage(msg *models.Message, remainder string) (broadcastPayload, error) {
	return payloadFromMessage(msg, remainder)
}

func payloadFromMessage(msg *models.Message, rawText string) (broadcastPayload, error) {
	text := strings.TrimSpace(rawText)

	if msg != nil && msg.ReplyToMessage != nil {
		return broadcastPayload{
			Type:            broadcastTypeCopy,
			SourceChatID:    msg.ReplyToMessage.Chat.ID,
			SourceMessageID: msg.ReplyToMessage.ID,
		}, nil
	}

	if msg != nil && strings.TrimSpace(msg.Caption) != "" {
		if text == "" {
			return broadcastPayload{}, fmt.Errorf("caption payload is empty")
		}
		return broadcastPayload{
			Type:            broadcastTypeCopy,
			Caption:         text,
			SourceChatID:    msg.Chat.ID,
			SourceMessageID: msg.ID,
		}, nil
	}

	if text == "" {
		return broadcastPayload{}, fmt.Errorf("text payload is empty")
	}
	return broadcastPayload{Type: broadcastTypeText, Text: text}, nil
}

func (b *Bot) ensureAdminCommand(msg *models.Message) bool {
	if b.isAdminMessage(msg) {
		return true
	}
	return b.rejectRestrictedCommand(msg)
}

func (b *Bot) ensureOwnerCommand(msg *models.Message) bool {
	if b.isOwnerMessage(msg) {
		return true
	}
	return b.rejectRestrictedCommand(msg)
}

func (b *Bot) rejectRestrictedCommand(msg *models.Message) bool {
	if msg != nil {
		b.send(msg.Chat.ID, commandUnavailableText)
	}
	return false
}
