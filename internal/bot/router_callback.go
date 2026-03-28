package bot

import "github.com/go-telegram/bot/models"

type callbackMeta struct {
	chatID int64
	msgID  int
	userID int64
	data   string
	sess   *Session
}

func (b *Bot) handleCallback(cq *models.CallbackQuery) {
	meta, ok := callbackMetaFromQuery(cq)
	if !ok {
		b.answer(cq, "Не удалось обработать callback.")
		return
	}

	b.logf("callback %s chat=%d user=%d msg=%d", logSnippet(meta.data, 64), meta.chatID, meta.userID, meta.msgID)

	switch meta.data {
	case cbPremiumBuy:
		b.answer(cq, b.handlePremiumPurchaseCallback(meta.chatID, meta.userID))
		return
	case "pl:all", "pl:select":
		b.answer(cq, "Этот режим больше недоступен. Выбери конкретные видео.")
		return
	case cbCancel:
		b.answer(cq, "")
		b.cancelSession(meta.chatID, b.sessions.get(meta.chatID))
		b.edit(meta.chatID, meta.msgID, "❌ Отменено.")
		return
	case cbNoop:
		b.answer(cq, "")
		return
	}

	sess, ok := b.callbackSession(meta)
	if !ok {
		b.answer(cq, staleSessionAlert)
		b.resetStaleSession(meta.chatID, meta.msgID)
		return
	}

	meta.sess = sess
	b.answer(cq, b.dispatchSessionCallback(meta))
}

func (b *Bot) callbackSession(meta callbackMeta) (*Session, bool) {
	sess := b.sessions.get(meta.chatID)
	if sessionState(sess) == StateIdle {
		return nil, false
	}
	return sess, true
}

func callbackMetaFromQuery(cq *models.CallbackQuery) (callbackMeta, bool) {
	chatID, msgID, ok := callbackMessageMeta(cq)
	if !ok {
		return callbackMeta{}, false
	}
	return callbackMeta{
		chatID: chatID,
		msgID:  msgID,
		userID: cq.From.ID,
		data:   cq.Data,
	}, true
}
