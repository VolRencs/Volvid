package bot

import "github.com/go-telegram/bot/models"

func (b *Bot) handleUpdate(update *models.Update) {
	switch {
	case update == nil:
		return
	case update.Message != nil:
		b.handleMessage(update.Message)
	case update.PreCheckoutQuery != nil:
		b.handlePreCheckout(update.PreCheckoutQuery)
	case update.CallbackQuery != nil:
		b.handleCallback(update.CallbackQuery)
	}
}
