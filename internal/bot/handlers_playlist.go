package bot

import (
	"strconv"
	"strings"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) handlePlaylistOpCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbPlVideo:
		sess.mutate(func(s *Session) {
			s.ForceSingle = true
			s.PlInfo = nil
			s.SelectedIndices = nil
			s.PlaylistPage = 0
			s.MediaDuration = 0
			s.Fragment = nil
			s.StatusMsgID = msgID
		})
		b.removeKb(chatID, msgID)
		b.probeAndAskFragment(chatID, sess)
	case cbPlChoose:
		sess.mutate(func(s *Session) {
			s.StatusMsgID = msgID
		})
		b.removeKb(chatID, msgID)
		b.fetchAndAskPlaylist(chatID, sess)
	}
	return ""
}

func (b *Bot) handlePlaylistSelectionInput(chatID int64, sess *Session, raw string) {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		b.resetStaleSession(chatID, 0)
		return
	}

	indices, err := app.ParseSelectionFor(raw, len(snap.PlInfo.Entries), app.LocaleRU)
	if err != nil {
		b.send(chatID, "⚠️ "+escapeHTML(err.Error())+"\n\nПример: <code>1-3,7,10</code> или <code>all</code>")
		return
	}

	if !b.selectPlaylistIndices(sess, indices) {
		b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
		return
	}
	b.askMode(chatID, sess)
}

func (b *Bot) handlePlaylistSelectionCallback(chatID int64, msgID int, sess *Session, data string) string {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return staleSessionAlert
	}

	switch {
	case data == cbPlSelectAll:
		if !b.selectAllPlaylistEntries(sess) {
			b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
			return b.playlistItemLimitAlert(snap.UserID)
		}
	case data == cbPlSelectNone:
		b.applyPlaylistSelection(sess, nil)
	case data == cbPlSelectDone:
		entries := b.playlistSelectionEntries(sess)
		if len(entries) == 0 {
			return "Выбери хотя бы одно видео."
		}
		if !b.validatePlaylistSelectionCount(snap.UserID, len(entries)) {
			b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
			return b.playlistItemLimitAlert(snap.UserID)
		}
		sess.mutate(func(s *Session) {
			s.StatusMsgID = msgID
		})
		b.askMode(chatID, sess)
		return ""
	case strings.HasPrefix(data, cbPlTogglePref):
		idx, err := strconv.Atoi(strings.TrimPrefix(data, cbPlTogglePref))
		if err != nil {
			return ""
		}
		selected, ok, overflow := b.togglePlaylistSelection(sess, idx)
		if overflow {
			b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
			return b.playlistItemLimitAlert(snap.UserID)
		}
		if !ok {
			return ""
		}
		b.applyPlaylistSelection(sess, selected)
	case strings.HasPrefix(data, cbPlPagePref):
		page, err := strconv.Atoi(strings.TrimPrefix(data, cbPlPagePref))
		if err != nil {
			return ""
		}
		sess.mutate(func(s *Session) {
			s.PlaylistPage = page
		})
	default:
		return ""
	}

	b.editKb(chatID, msgID, b.playlistSelectionText(sess), b.kbPlaylistSelection(sess))
	return ""
}
