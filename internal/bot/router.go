package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

const staleSessionAlert = "Сессия устарела. Пришли ссылку заново."

type callbackMeta struct {
	chatID int64
	msgID  int
	userID int64
	data   string
	sess   *Session
}

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

	b.trackKnownUser(msg, userID)
	if b.handleSuccessfulPayment(msg) {
		return
	}

	sess := b.sessions.get(chatID)
	if b.handleSessionTextInput(msg, chatID, userID, sess, text) {
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

func (b *Bot) trackKnownUser(msg *models.Message, userID int64) {
	if !isPrivateChatMessage(msg) || b.users == nil {
		return
	}
	if err := b.users.Add(userID); err != nil {
		b.logError("users add", err)
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
		if !b.cancelCommandSession(chatID) {
			b.send(chatID, "Нечего отменять.")
			return
		}
		b.send(chatID, "Отменено.")
	case "status":
		if !b.ensureAdminCommand(msg) {
			return
		}
		deps := app.DetectDeps()
		b.send(chatID, "<b>Зависимости</b>\n\n"+
			depLine(deps.YTDLP)+"\n"+
			depLine(deps.FFmpeg)+"\n"+
			depLine(deps.Node)+"\n"+
			accessLine("browser cookies", deps.Cookies.Status, cookiesDetail(deps.Cookies))+"\n"+
			accessLine("js runtime", deps.Runtime.Status, runtimeDetail(deps.Runtime))+"\n"+
			verLine("bot api", b.backendLabel()+" | "+b.cfg.ServerURL))
	case "update":
		if !b.ensureOwnerCommand(msg) {
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

func (b *Bot) cancelCommandSession(chatID int64) bool {
	sess := b.sessions.get(chatID)
	if sessionState(sess) == StateIdle {
		return false
	}
	b.cancelSession(chatID, sess)
	return true
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
		b.edit(meta.chatID, meta.msgID, "Отменено.")
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
	if !sessionOwnedBy(sess, meta.userID) {
		b.answer(cq, "Эта сессия принадлежит другому пользователю.")
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

func (b *Bot) dispatchSessionCallback(meta callbackMeta) string {
	switch sessionState(meta.sess) {
	case StateAwaitingSearchSelection:
		return b.handleSearchCallback(meta.chatID, meta.sess, meta.data, meta.userID)
	case StateAwaitingPlaylistOp:
		return b.handlePlaylistOpCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
	case StateAwaitingFragmentChoice:
		return b.handleFragmentChoiceCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
	case StateAwaitingPlaylistSelection:
		return b.handlePlaylistSelectionCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
	case StateAwaitingMode:
		return b.handleModeCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
	case StateAwaitingAudioProfile:
		return b.handleAudioProfileCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
	case StateAwaitingQuality:
		return b.handleQualityCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
	default:
		return ""
	}
}

func (b *Bot) handleSessionTextInput(msg *models.Message, chatID, userID int64, sess *Session, text string) bool {
	if messageIsCommand(msg) || sessionState(sess) == StateIdle || !sessionOwnedBy(sess, userID) || strings.TrimSpace(text) == "" || app.LooksLikeYouTubeURL(text) {
		return false
	}

	switch sessionState(sess) {
	case StateAwaitingFragmentInput:
		b.handleFragmentInput(chatID, sess, text)
		return true
	case StateAwaitingPlaylistSelection:
		b.handlePlaylistSelectionInput(chatID, sess, text)
		return true
	default:
		return false
	}
}

func sessionState(sess *Session) UserState {
	if sess == nil {
		return StateIdle
	}
	return sess.snapshot().State
}

func sessionOwnedBy(sess *Session, userID int64) bool {
	if sess == nil {
		return false
	}
	ownerID := sess.snapshot().UserID
	return ownerID == 0 || ownerID == userID
}

func (b *Bot) cancelSession(chatID int64, sess *Session) {
	if sess != nil {
		sess.cancel()
	}
	b.sessions.reset(chatID)
}

func (b *Bot) resetStaleSession(chatID int64, msgID int) {
	b.replace(chatID, msgID, staleSessionAlert)
	b.sessions.reset(chatID)
}
