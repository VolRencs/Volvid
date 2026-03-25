package bot

import app "YouTubeBuild/internal/app"

const activeDownloadText = "⏳ Уже идёт скачивание. Дождись завершения или отмени /cancel."

func (b *Bot) rejectWhileDownloading(chatID int64) bool {
	sess := b.sessions.get(chatID)
	if sess == nil || sess.snapshot().State != StateDownloading {
		return false
	}
	snap := sess.snapshot()
	b.logf("rejecting new request while download active chat=%d user=%d", chatID, snap.UserID)
	b.send(chatID, activeDownloadText)
	return true
}

func (b *Bot) startConfiguredDownloadAlert(chatID int64, sess *Session) string {
	if err := b.startConfiguredDownload(chatID, sess); err != nil {
		if err == errDownloadLimitExceeded {
			return ""
		}
		return err.Error()
	}
	return ""
}

func (b *Bot) setSessionDownloadChoice(sess *Session, state UserState, mode app.DownloadMode, profile app.OutputProfile, msgID int) {
	sess.mutate(func(s *Session) {
		s.State = state
		s.Mode = mode
		s.Profile = profile
		s.StatusMsgID = msgID
	})
}
