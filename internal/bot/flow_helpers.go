package bot

const activeDownloadText = "⏳ Уже идёт скачивание. Дождись завершения или отмени /cancel."

func (b *Bot) rejectWhileDownloading(chatID int64) bool {
	sess := b.sessions.get(chatID)
	if sessionState(sess) != StateDownloading {
		return false
	}
	snap := sess.snapshot()
	b.logf("rejecting new request while download active chat=%d user=%d", chatID, snap.UserID)
	b.send(chatID, activeDownloadText)
	return true
}
