package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

type sessionTextHandler func(chatID int64, sess *Session, text string)
type sessionCallbackHandler func(meta callbackMeta) string

const staleSessionAlert = "Сессия устарела. Пришли ссылку заново."

func (b *Bot) dispatchSessionCallback(meta callbackMeta) string {
	handler := b.sessionCallbackHandler(sessionState(meta.sess))
	if handler == nil {
		return ""
	}
	return handler(meta)
}

func (b *Bot) handleSessionTextInput(msg *models.Message, chatID int64, sess *Session, text string) bool {
	if messageIsCommand(msg) || sessionState(sess) == StateIdle || strings.TrimSpace(text) == "" || app.LooksLikeYouTubeURL(text) {
		return false
	}

	handler := b.sessionTextHandler(sessionState(sess))
	if handler == nil {
		return false
	}
	handler(chatID, sess, text)
	return true
}

func (b *Bot) sessionTextHandler(state UserState) sessionTextHandler {
	switch state {
	case StateAwaitingFragmentInput:
		return b.handleFragmentInput
	case StateAwaitingPlaylistSelection:
		return b.handlePlaylistSelectionInput
	default:
		return nil
	}
}

func (b *Bot) sessionCallbackHandler(state UserState) sessionCallbackHandler {
	switch state {
	case StateAwaitingSearchSelection:
		return func(meta callbackMeta) string {
			return b.handleSearchCallback(meta.chatID, meta.sess, meta.data, meta.userID)
		}
	case StateAwaitingPlaylistOp:
		return func(meta callbackMeta) string {
			return b.handlePlaylistOpCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
		}
	case StateAwaitingFragmentChoice:
		return func(meta callbackMeta) string {
			return b.handleFragmentChoiceCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
		}
	case StateAwaitingPlaylistSelection:
		return func(meta callbackMeta) string {
			return b.handlePlaylistSelectionCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
		}
	case StateAwaitingMode:
		return func(meta callbackMeta) string {
			return b.handleModeCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
		}
	case StateAwaitingAudioProfile:
		return func(meta callbackMeta) string {
			return b.handleAudioProfileCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
		}
	case StateAwaitingQuality:
		return func(meta callbackMeta) string {
			return b.handleQualityCallback(meta.chatID, meta.msgID, meta.sess, meta.data)
		}
	default:
		return nil
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
