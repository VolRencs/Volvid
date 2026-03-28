package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) apiCtx() (context.Context, context.CancelFunc) {
	return timeoutCtx(telegramAPITimeout)
}

func (b *Bot) fileSendCtx() (context.Context, context.CancelFunc) {
	return timeoutCtx(telegramFileSendTimeout)
}

func (b *Bot) send(chatID int64, text string) (models.Message, error) {
	return b.sendWithKeyboard(chatID, text, nil)
}

func (b *Bot) sendKb(chatID int64, text string, kb models.InlineKeyboardMarkup) (models.Message, error) {
	return b.sendWithKeyboard(chatID, text, &kb)
}

func (b *Bot) sendWithKeyboard(chatID int64, text string, kb *models.InlineKeyboardMarkup) (models.Message, error) {
	ctx, cancel := b.apiCtx()
	defer cancel()

	params := &tg.SendMessageParams{
		ChatID:             chatID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: disabledPreview(),
	}
	if kb != nil {
		params.ReplyMarkup = *kb
	}

	msg, err := b.api.SendMessage(ctx, params)
	if err != nil {
		logTelegramActionError(fmt.Sprintf("sendMessage chat=%d", chatID), err)
		return models.Message{}, err
	}
	return *msg, nil
}

func (b *Bot) edit(chatID int64, msgID int, text string) {
	b.editWithKeyboard(chatID, msgID, text, nil)
}

func (b *Bot) editKb(chatID int64, msgID int, text string, kb models.InlineKeyboardMarkup) {
	b.editWithKeyboard(chatID, msgID, text, &kb)
}

func (b *Bot) editWithKeyboard(chatID int64, msgID int, text string, kb *models.InlineKeyboardMarkup) {
	ctx, cancel := b.apiCtx()
	defer cancel()

	params := &tg.EditMessageTextParams{
		ChatID:             chatID,
		MessageID:          msgID,
		Text:               text,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: disabledPreview(),
	}
	if kb != nil {
		params.ReplyMarkup = *kb
	}

	if _, err := b.api.EditMessageText(ctx, params); err != nil && !isMessageNotModified(err) {
		logTelegramActionError(fmt.Sprintf("editMessageText chat=%d msg=%d", chatID, msgID), err)
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
		logTelegramActionError(fmt.Sprintf("editMessageReplyMarkup chat=%d msg=%d", chatID, msgID), err)
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
		logTelegramActionError(fmt.Sprintf("answerCallbackQuery id=%s", cq.ID), err)
	}
}

func logTelegramActionError(action string, err error) {
	if err == nil {
		return
	}
	log.Printf("telegram %s: %v", action, err)
}

func isMessageNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "message is not modified")
}
