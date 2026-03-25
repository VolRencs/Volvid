package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

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

func (b *Bot) handleMessage(msg *models.Message) {
	chatID := msg.Chat.ID
	userID := senderID(msg)
	text, _ := commandTextAndEntities(msg)
	text = strings.TrimSpace(text)

	if isPrivateChatMessage(msg) && b.users != nil {
		if err := b.users.Add(userID); err != nil {
			b.logError("users add", err)
		}
	}
	if b.handleSuccessfulPayment(msg) {
		return
	}

	sess := b.sessions.get(chatID)
	snap := sess.snapshot()
	if !messageIsCommand(msg) && snap.State == StateAwaitingPlaylistSelection && text != "" && !app.LooksLikeYouTubeURL(text) {
		b.handlePlaylistSelectionInput(chatID, sess, text)
		return
	}

	switch {
	case messageIsCommand(msg):
		b.handleCommand(msg)
	case app.LooksLikeYouTubeURL(text):
		b.handleURL(chatID, userID, text)
	case text != "":
		b.handleSearch(chatID, userID, text)
	}
}

func (b *Bot) handleCommand(msg *models.Message) {
	chatID := msg.Chat.ID
	cmd := messageCommand(msg)

	b.logf("command %s %s", cmd, logChatUser(chatID, senderID(msg)))

	switch cmd {
	case "start", "help":
		b.send(chatID, b.helpText(msg))
	case "cancel":
		sess := b.sessions.get(chatID)
		if sess == nil || sess.snapshot().State == StateIdle {
			b.send(chatID, "Нечего отменять.")
			return
		}
		sess.cancel()
		b.sessions.reset(chatID)
		b.send(chatID, "❌ Отменено.")
	case "status":
		if !b.isAdminMessage(msg) {
			b.send(chatID, "Команда недоступна.")
			return
		}
		deps := app.DetectDeps()
		b.send(chatID, "⚙️ <b>Зависимости</b>\n\n"+
			verLine("yt-dlp", deps.YtdlpVer)+"\n"+
			verLine("ffmpeg", deps.FFmpegVer)+"\n"+
			verLine("bot api", b.backendLabel()+" | "+b.cfg.ServerURL))
	case "update":
		if !b.isOwnerMessage(msg) {
			b.send(chatID, "Команда недоступна.")
			return
		}
		b.startDepsUpdate(chatID)
	case "premium":
		b.handlePremiumCommand(chatID, senderID(msg))
	case "broadcast":
		b.handleBroadcastCommand(msg)
	case "schedule":
		b.handleScheduleCommand(msg)
	case "timers":
		b.handleTimersCommand(msg)
	case "deltimer":
		b.handleDelTimerCommand(msg)
	}
}

func (b *Bot) handleCallback(cq *models.CallbackQuery) {
	chatID, msgID, ok := callbackMessageMeta(cq)
	if !ok {
		b.answer(cq, "Не удалось обработать callback.")
		return
	}

	data := cq.Data
	userID := cq.From.ID
	b.logf("callback %s chat=%d user=%d msg=%d", logSnippet(data, 64), chatID, userID, msgID)
	if data == cbPremiumBuy {
		b.answer(cq, b.handlePremiumPurchaseCallback(chatID, userID))
		return
	}
	if data == "pl:all" || data == "pl:select" {
		b.answer(cq, "Этот режим больше недоступен. Выбери конкретные видео.")
		return
	}

	sess := b.sessions.get(chatID)
	if sess == nil {
		b.answer(cq, "Сессия устарела. Пришли ссылку заново.")
		b.edit(chatID, msgID, "⚠️ Сессия устарела. Пришли ссылку заново.")
		return
	}

	if data == cbCancel {
		b.answer(cq, "")
		sess.cancel()
		b.sessions.reset(chatID)
		b.edit(chatID, msgID, "❌ Отменено.")
		return
	}
	if data == cbNoop {
		b.answer(cq, "")
		return
	}

	var alert string
	switch sess.snapshot().State {
	case StateAwaitingSearchSelection:
		alert = b.handleSearchCallback(chatID, sess, data, userID)
	case StateAwaitingPlaylistOp:
		alert = b.handlePlaylistOpCallback(chatID, msgID, sess, data)
	case StateAwaitingPlaylistSelection:
		alert = b.handlePlaylistSelectionCallback(chatID, msgID, sess, data)
	case StateAwaitingMode:
		alert = b.handleModeCallback(chatID, msgID, sess, data)
	case StateAwaitingAudioProfile:
		alert = b.handleAudioProfileCallback(chatID, msgID, sess, data)
	case StateAwaitingQuality:
		alert = b.handleQualityCallback(chatID, msgID, sess, data)
	}
	b.answer(cq, alert)
}
