package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

const staleSessionAlert = "Сессия устарела. Пришли ссылку заново."

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

func (b *Bot) handleSessionTextInput(msg *models.Message, chatID int64, sess *Session, text string) bool {
	if messageIsCommand(msg) || sessionState(sess) == StateIdle || strings.TrimSpace(text) == "" || app.LooksLikeYouTubeURL(text) {
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

func (b *Bot) cancelSession(chatID int64, sess *Session) {
	if sess != nil {
		sess.cancel()
	}
	b.sessions.reset(chatID)
}

func (b *Bot) resetStaleSession(chatID int64, msgID int) {
	text := "⚠️ " + staleSessionAlert
	if msgID != 0 {
		b.edit(chatID, msgID, text)
	} else {
		b.send(chatID, text)
	}
	b.sessions.reset(chatID)
}
