package bot

import (
	"fmt"
	"strconv"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const premiumCurrency = "XTR"

func (b *Bot) handlePremiumCommand(chatID, userID int64) {
	if b.hasPremium(userID) {
		b.send(chatID, "⭐ Премиум уже активен.\n\nТвой лимит загрузки: 2GB.\nЛимит плейлиста: до 30 видео.")
		return
	}

	b.sendKb(chatID,
		fmt.Sprintf("⭐ <b>Премиум</b>\n\nЛимит загрузки: 2GB\nЛимит плейлиста: до 30 видео\nЦена: %d XTR\n\nНажми кнопку ниже, чтобы оплатить Telegram Stars.", b.cfg.PremiumStarsPrice),
		kbPremiumOffer(b.cfg.PremiumStarsPrice),
	)
}

func (b *Bot) handlePremiumPurchaseCallback(chatID, userID int64) string {
	if b.hasPremium(userID) {
		b.send(chatID, "⭐ Премиум уже активен.")
		return ""
	}
	if err := b.sendPremiumInvoice(chatID, userID); err != nil {
		return "Не удалось создать счёт на оплату."
	}
	return ""
}

func (b *Bot) sendPremiumInvoice(chatID, userID int64) error {
	ctx, cancel := b.apiCtx()
	defer cancel()

	_, err := b.api.SendInvoice(ctx, &tg.SendInvoiceParams{
		ChatID:         chatID,
		Title:          "VolRen Premium",
		Description:    "Lifetime-премиум с лимитом загрузки до 2GB и до 30 видео из плейлиста.",
		Payload:        premiumPayload(userID, b.cfg.PremiumStarsPrice),
		Currency:       premiumCurrency,
		Prices:         []models.LabeledPrice{{Label: "Premium", Amount: b.cfg.PremiumStarsPrice}},
		ProviderToken:  "",
		StartParameter: "volren-premium",
	})
	return err
}

func (b *Bot) handlePreCheckout(query *models.PreCheckoutQuery) {
	if query == nil {
		return
	}

	ok := false
	errText := "Не удалось подтвердить оплату."
	if query.Currency == premiumCurrency {
		if userID, price, valid := parsePremiumPayload(query.InvoicePayload); valid && userID == query.From.ID && price == query.TotalAmount && price == b.cfg.PremiumStarsPrice {
			ok = true
			errText = ""
		} else {
			errText = "Некорректные параметры оплаты премиума."
		}
	} else {
		errText = "Поддерживается только оплата в Telegram Stars."
	}

	ctx, cancel := b.apiCtx()
	defer cancel()
	if _, err := b.api.AnswerPreCheckoutQuery(ctx, &tg.AnswerPreCheckoutQueryParams{
		PreCheckoutQueryID: query.ID,
		OK:                 ok,
		ErrorMessage:       errText,
	}); err != nil {
		return
	}
}

func (b *Bot) handleSuccessfulPayment(msg *models.Message) bool {
	if msg == nil || msg.SuccessfulPayment == nil {
		return false
	}

	payment := msg.SuccessfulPayment
	userID := senderID(msg)
	payloadUserID, price, valid := parsePremiumPayload(payment.InvoicePayload)
	if !valid || payloadUserID != userID || payment.Currency != premiumCurrency || payment.TotalAmount != price || price != b.cfg.PremiumStarsPrice {
		b.send(msg.Chat.ID, "⚠️ Платёж получен, но параметры премиума не совпали. Напиши администратору.")
		return true
	}

	if err := b.premium.AddPremium(userID); err != nil {
		b.send(msg.Chat.ID, "⚠️ Платёж прошёл, но не удалось активировать премиум автоматически. Напиши администратору.")
		return true
	}

	b.send(msg.Chat.ID, "⭐ Премиум активирован.\n\nНовый лимит загрузки: 2GB.\nНовый лимит плейлиста: до 30 видео.")
	return true
}

func premiumPayload(userID int64, price int) string {
	return fmt.Sprintf("premium:%d:%d", userID, price)
}

func parsePremiumPayload(payload string) (userID int64, price int, ok bool) {
	parts := strings.Split(strings.TrimSpace(payload), ":")
	if len(parts) != 3 || parts[0] != "premium" {
		return 0, 0, false
	}
	uid, errUID := strconv.ParseInt(parts[1], 10, 64)
	amount, errAmount := strconv.Atoi(parts[2])
	if errUID != nil || errAmount != nil || uid == 0 || amount <= 0 {
		return 0, 0, false
	}
	return uid, amount, true
}

func kbPremiumOffer(price int) models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				kbButton(fmt.Sprintf("⭐ Купить премиум · %d XTR", price), cbPremiumBuy),
			},
		},
	}
}
