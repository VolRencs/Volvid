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
			s.StatusMsgID = msgID
		})
		b.removeKb(chatID, msgID)
		b.askMode(chatID, sess)
	case cbPlFull:
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
		b.send(chatID, "⚠️ Сессия устарела. Пришли ссылку заново.")
		b.sessions.reset(chatID)
		return
	}

	indices, err := app.ParseSelectionFor(raw, len(snap.PlInfo.Entries), app.LocaleRU)
	if err != nil {
		b.send(chatID, "⚠️ "+escapeHTML(err.Error())+"\n\nПример: <code>1-3,7,10</code> или <code>all</code>")
		return
	}

	selected := selectionMapFromIndices(indices)
	if !b.validatePlaylistSelectionCount(snap.UserID, len(selected)) {
		b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
		return
	}
	b.applyPlaylistSelection(sess, selected)
	b.askMode(chatID, sess)
}

func (b *Bot) handlePlaylistSelectionCallback(chatID int64, msgID int, sess *Session, data string) string {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return "Сессия устарела. Пришли ссылку заново."
	}

	switch {
	case data == cbPlSelectAll:
		selected := selectionMapForAll(snap.PlInfo.Entries)
		if !b.validatePlaylistSelectionCount(snap.UserID, len(selected)) {
			b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
			return b.playlistItemLimitAlert(snap.UserID)
		}
		b.applyPlaylistSelection(sess, selected)
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

func (b *Bot) handleModeCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbModeVideo:
		sess.mutate(func(s *Session) {
			s.Mode = app.ModeVideo
			s.Profile = app.OutputProfile{}
			s.StatusMsgID = msgID
		})
		b.scanAndAskQuality(chatID, sess)
	case cbModeAudio:
		sess.mutate(func(s *Session) {
			s.Mode = app.ModeAudio
			s.Profile = app.OutputProfile{}
			s.StatusMsgID = msgID
		})
		b.askAudioProfiles(chatID, sess)
	case cbModeThumb:
		sess.mutate(func(s *Session) {
			s.Mode = app.ModeThumbnail
			s.Profile = app.ThumbnailOutputProfile(app.LocaleRU)
			s.StatusMsgID = msgID
		})
		if err := b.startConfiguredDownload(chatID, sess); err != nil {
			if err == errDownloadLimitExceeded {
				return ""
			}
			return err.Error()
		}
	}
	return ""
}

func (b *Bot) handleAudioProfileCallback(chatID int64, msgID int, sess *Session, data string) string {
	if !strings.HasPrefix(data, cbAudioPrefix) {
		return ""
	}

	profiles := app.AudioOutputProfiles(app.LocaleRU)
	profile, ok := app.FindOutputProfile(profiles, strings.TrimPrefix(data, cbAudioPrefix))
	if !ok {
		return ""
	}

	sess.mutate(func(s *Session) {
		s.Profile = profile
		s.StatusMsgID = msgID
	})
	if err := b.startConfiguredDownload(chatID, sess); err != nil {
		if err == errDownloadLimitExceeded {
			return ""
		}
		return err.Error()
	}
	return ""
}

func (b *Bot) handleQualityCallback(chatID int64, msgID int, sess *Session, data string) string {
	if !strings.HasPrefix(data, cbQualityPrefix) {
		return ""
	}
	choices := sess.snapshot().QualityChoices
	if len(choices) == 0 {
		choices = app.DefaultQualityChoices()
	}
	choice, ok := app.FindQualityChoice(choices, strings.TrimPrefix(data, cbQualityPrefix))
	if !ok {
		return ""
	}

	sess.mutate(func(s *Session) {
		s.Profile = choice.Profile(app.LocaleRU)
		s.StatusMsgID = msgID
	})
	if err := b.startConfiguredDownload(chatID, sess); err != nil {
		if err == errDownloadLimitExceeded {
			return ""
		}
		return err.Error()
	}
	return ""
}
