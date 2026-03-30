package bot

import (
	"fmt"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) fetchAndAskPlaylist(chatID int64, sess *Session) {
	sess.beginPlaylistFetch()

	snap := sess.snapshot()
	b.logf("fetch playlist %s url=%q", logChatUser(chatID, snap.UserID), logSnippet(snap.URL, 200))
	b.upsertSessionText(chatID, sess, "⏳ Загружаю список плейлиста…")

	go b.runPlaylistFetch(chatID, sess)
}

func (b *Bot) runPlaylistFetch(chatID int64, sess *Session) {
	snap := sess.snapshot()
	info, err := app.FetchPlaylistInfoFor(nil, snap.URL, app.LocaleRU)
	if sess.isCancelled() {
		b.logf("playlist fetch cancelled %s", logChatUser(chatID, snap.UserID))
		return
	}
	if err != nil || info == nil || len(info.Entries) == 0 {
		b.logError(fmt.Sprintf("playlist fetch %s", logChatUser(chatID, snap.UserID)), err)
		b.failPlaylistSelection(chatID, sess)
		return
	}

	b.logf("playlist loaded %s title=%q entries=%d", logChatUser(chatID, snap.UserID), logSnippet(info.Title, 80), len(info.Entries))
	sess.storePlaylist(info)

	state := sess.snapshot()
	b.openPlaylistSelection(chatID, state.StatusMsgID, sess)
}

func (b *Bot) failPlaylistSelection(chatID int64, sess *Session) {
	snap := sess.snapshot()
	text := "⚠️ Не удалось загрузить список плейлиста. Пришли ссылку заново."
	b.replace(chatID, snap.StatusMsgID, text)
	b.sessions.reset(chatID)
}
