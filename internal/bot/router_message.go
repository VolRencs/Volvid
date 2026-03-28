package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

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
	if b.handleSessionTextInput(msg, chatID, sess, text) {
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

func (b *Bot) cancelCommandSession(chatID int64) bool {
	sess := b.sessions.get(chatID)
	if sessionState(sess) == StateIdle {
		return false
	}
	b.cancelSession(chatID, sess)
	return true
}
