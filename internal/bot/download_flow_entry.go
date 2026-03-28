package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) handleURL(chatID, userID int64, rawURL string) {
	target, ok := b.parseURLTarget(chatID, userID, rawURL)
	if !ok {
		return
	}
	if !b.canStartDownloadFlow(chatID, userID) {
		return
	}

	sess, ok := b.createDownloadSession(chatID, userID, rawURL, target)
	if !ok {
		return
	}

	b.openDownloadTargetFlow(chatID, sess, target)
}

func (b *Bot) parseURLTarget(chatID, userID int64, rawURL string) (app.ParsedTarget, bool) {
	target, err := app.ParseTarget(rawURL)
	if err != nil {
		b.logf("bad url %s url=%q", logChatUser(chatID, userID), logSnippet(rawURL, 200))
		b.send(chatID, "⚠️ Не удалось распознать YouTube-ссылку.")
		return app.ParsedTarget{}, false
	}

	b.logf("url accepted %s kind=%s url=%q", logChatUser(chatID, userID), target.Kind.String(), logSnippet(target.CanonicalURL, 200))
	return target, true
}

func (b *Bot) canStartDownloadFlow(chatID, userID int64) bool {
	if b.rejectWhileDownloading(chatID) {
		return false
	}
	deps := app.DetectDeps()
	if !deps.MissingRequired() {
		return true
	}

	missing := "обязательные зависимости"
	if names := deps.MissingRequiredDeps(); len(names) > 0 {
		labels := make([]string, 0, len(names))
		for _, dep := range names {
			if dep.Name != "" {
				labels = append(labels, dep.Name)
			}
		}
		if len(labels) > 0 {
			missing = strings.Join(labels, ", ")
		}
	}
	b.logf("dependencies unavailable %s missing=%q", logChatUser(chatID, userID), missing)
	b.send(chatID,
		"⚠️ <b>Недоступны обязательные зависимости.</b>\n\n"+
			"Не найдены: <code>"+escapeHTML(missing)+"</code>\n\n"+
			"Бот не смог автоматически подготовить зависимости на сервере.")
	return false
}

func (b *Bot) createDownloadSession(chatID, userID int64, rawURL string, target app.ParsedTarget) (*Session, bool) {
	workDir, err := createBotWorkDir(userID)
	if err != nil {
		b.logError("create bot workdir", err)
		b.send(chatID, "⚠️ Не удалось подготовить временную папку для загрузки.")
		return nil, false
	}

	sess := newSession(userID, rawURL, workDir)
	sess.mutate(func(s *Session) {
		s.Target = target
		s.MediaDuration = 0
		s.Fragment = nil
	})
	b.sessions.set(chatID, sess)
	return sess, true
}

func (b *Bot) openDownloadTargetFlow(chatID int64, sess *Session, target app.ParsedTarget) {
	switch {
	case target.Kind == app.TargetMixed:
		sess.mutate(func(s *Session) {
			s.State = StateAwaitingPlaylistOp
		})
		b.sendKb(chatID, "⚠️ Ссылка содержит и видео, и плейлист. Что скачать?", kbPlaylistChoice())
	case target.IsPlaylist():
		b.fetchAndAskPlaylist(chatID, sess)
	default:
		b.probeAndAskFragment(chatID, sess)
	}
}
