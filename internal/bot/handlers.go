package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) handleMessage(msg *models.Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)
	sess := b.sessions.get(chatID)
	snap := sess.snapshot()

	if !messageIsCommand(msg) && snap.State == StateAwaitingPlaylistSelection && text != "" && !app.LooksLikeYouTubeURL(text) {
		b.handlePlaylistSelectionInput(chatID, sess, text)
		return
	}

	switch {
	case messageIsCommand(msg):
		b.handleCommand(msg)
	case app.LooksLikeYouTubeURL(text):
		b.handleURL(chatID, text)
	case text != "":
		b.send(chatID,
			"🔗 Пришли ссылку на YouTube-видео или плейлист.\n\n"+
				"Поддерживаются:\n"+
				"• <code>youtube.com/watch?v=…</code>\n"+
				"• <code>youtube.com/playlist?list=…</code>\n"+
				"• <code>youtu.be/…</code>",
		)
	}
}

func (b *Bot) handleCommand(msg *models.Message) {
	chatID := msg.Chat.ID
	switch messageCommand(msg) {
	case "start", "help":
		b.send(chatID, b.helpText(msg))
	case "cancel":
		sess := b.sessions.get(chatID)
		if sess == nil || sess.snapshot().State == StateIdle {
			b.send(chatID, "Нечего отменять.")
			return
		}
		sess.cancel()
		b.sessions.reset(chatID)
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
	}
}

func (b *Bot) handleURL(chatID int64, rawURL string) {
	target, err := app.ParseTarget(rawURL)
	if err != nil {
		b.send(chatID, "⚠️ Не удалось распознать YouTube-ссылку.")
		return
	}

	sess := b.sessions.get(chatID)
	if sess != nil && sess.snapshot().State == StateDownloading {
		b.send(chatID, "⏳ Уже идёт скачивание. Дождись завершения или отмени /cancel.")
		return
	}

	if app.YtdlpVersion() == "" {
		b.send(chatID,
			"⚠️ <b>yt-dlp недоступен.</b>\n\n"+
				"Бот не смог автоматически подготовить зависимости на сервере.")
		return
	}

	workDir, err := createBotWorkDir(chatID)
	if err != nil {
		b.send(chatID, "⚠️ Не удалось подготовить временную папку для загрузки.")
		return
	}

	newSess := newSession(rawURL, workDir)
	newSess.mutate(func(s *Session) {
		s.Target = target
	})
	b.sessions.set(chatID, newSess)

	if target.Kind == app.TargetMixed {
		newSess.mutate(func(s *Session) {
			s.State = StateAwaitingPlaylistOp
		})
		b.sendKb(chatID, "⚠️ Ссылка содержит и видео, и плейлист. Что скачать?", kbPlaylistChoice())
		return
	}

	if target.IsPlaylist() {
		b.fetchAndAskPlaylist(chatID, newSess)
		return
	}

	b.askMode(chatID, newSess)
}

func (b *Bot) fetchAndAskPlaylist(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateFetchingPlaylist
	})
	statusMsg, _ := b.send(chatID, "⏳ Загружаю список плейлиста…")
	sess.mutate(func(s *Session) {
		s.StatusMsgID = statusMsg.ID
	})

	go func() {
		snap := sess.snapshot()
		info, err := app.FetchPlaylistInfoFor(nil, snap.URL, app.LocaleRU)
		if sess.isCancelled() {
			return
		}
		if err != nil || info == nil {
			sess.mutate(func(s *Session) {
				s.ForceSingle = true
				s.SelectedEntries = nil
			})
			b.askMode(chatID, sess)
			return
		}
		sess.mutate(func(s *Session) {
			s.PlInfo = info
			s.SelectedEntries = nil
			s.SelectedIndices = nil
			s.PlaylistPage = 0
			s.QualityChoices = nil
			s.Profile = app.OutputProfile{}
			s.State = StateAwaitingPlaylistScope
		})
		state := sess.snapshot()
		b.editKb(chatID, state.StatusMsgID, b.playlistScopeText(sess), kbPlaylistScope())
	}()
}

func (b *Bot) scanAndAskQuality(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateFetchingQuality
	})
	state := sess.snapshot()
	if state.StatusMsgID == 0 {
		statusMsg, _ := b.send(chatID, b.qualityScanText(sess))
		sess.mutate(func(s *Session) {
			s.StatusMsgID = statusMsg.ID
		})
	} else {
		b.edit(chatID, state.StatusMsgID, b.qualityScanText(sess))
	}

	urls := b.qualityScanURLs(sess)
	go func() {
		choices, _ := app.ResolveQualityChoices(urls)
		if sess.isCancelled() {
			return
		}
		if len(choices) == 0 {
			choices = app.DefaultQualityChoices()
		}
		sess.mutate(func(s *Session) {
			s.QualityChoices = choices
			s.State = StateAwaitingQuality
		})
		state := sess.snapshot()
		b.editKb(chatID, state.StatusMsgID, b.qualityPromptText(sess), kbQuality(choices))
	}()
}

func (b *Bot) askMode(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateAwaitingMode
		s.Mode = app.DefaultDownloadMode()
		s.Profile = app.DefaultVideoProfile(app.LocaleRU)
		s.QualityChoices = nil
	})
	snap := sess.snapshot()
	if snap.StatusMsgID == 0 {
		msg, _ := b.sendKb(chatID, b.modePromptText(sess), kbMode())
		sess.mutate(func(s *Session) { s.StatusMsgID = msg.ID })
		return
	}
	b.editKb(chatID, snap.StatusMsgID, b.modePromptText(sess), kbMode())
}

func (b *Bot) askAudioProfiles(chatID int64, sess *Session) {
	profiles := app.AudioOutputProfiles(app.LocaleRU)
	sess.mutate(func(s *Session) {
		s.State = StateAwaitingAudioProfile
		s.Profile = app.OutputProfile{}
	})
	snap := sess.snapshot()
	b.editKb(chatID, snap.StatusMsgID, b.audioPromptText(sess), kbAudioProfiles(profiles))
}

func (b *Bot) startConfiguredDownload(chatID int64, sess *Session) error {
	req, err := b.buildDownloadRequest(sess)
	if err != nil {
		return err
	}

	sess.mutate(func(s *Session) {
		s.State = StateDownloading
	})

	snap := sess.snapshot()
	desc := b.downloadStartText(sess)
	b.removeKb(chatID, snap.StatusMsgID)
	b.edit(chatID, snap.StatusMsgID, desc)
	go b.runDownload(chatID, sess, req)
	return nil
}

func (b *Bot) handleCallback(cq *models.CallbackQuery) {
	chatID, msgID, ok := callbackMessageMeta(cq)
	if !ok {
		b.answer(cq, "Не удалось обработать callback.")
		return
	}
	data := cq.Data

	sess := b.sessions.get(chatID)
	if sess == nil {
		b.answer(cq, "Сессия устарела. Пришли ссылку заново.")
		b.edit(chatID, msgID, "⚠️ Сессия устарела. Пришли ссылку заново.")
		return
	}

	if data == cbCancel {
		b.answer(cq, "")
		sess.cancel()
		b.sessions.reset(chatID)
		b.edit(chatID, msgID, "❌ Отменено.")
		return
	}
	if data == cbNoop {
		b.answer(cq, "")
		return
	}

	var alert string
	switch sess.snapshot().State {
	case StateAwaitingPlaylistOp:
		alert = b.handlePlaylistOpCallback(chatID, msgID, sess, data)
	case StateAwaitingPlaylistScope:
		alert = b.handlePlaylistScopeCallback(chatID, msgID, sess, data)
	case StateAwaitingPlaylistSelection:
		alert = b.handlePlaylistSelectionCallback(chatID, msgID, sess, data)
	case StateAwaitingMode:
		alert = b.handleModeCallback(chatID, msgID, sess, data)
	case StateAwaitingAudioProfile:
		alert = b.handleAudioProfileCallback(chatID, msgID, sess, data)
	case StateAwaitingQuality:
		alert = b.handleQualityCallback(chatID, msgID, sess, data)
	}
	b.answer(cq, alert)
}

func (b *Bot) handlePlaylistOpCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbPlVideo:
		sess.mutate(func(s *Session) {
			s.ForceSingle = true
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

func (b *Bot) handlePlaylistScopeCallback(chatID int64, msgID int, sess *Session, data string) string {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return "Сессия устарела. Пришли ссылку заново."
	}
	switch data {
	case cbPlAll:
		sess.mutate(func(s *Session) {
			s.SelectedEntries = append([]app.PlaylistEntry(nil), snap.PlInfo.Entries...)
			s.SelectedIndices = nil
			s.PlaylistPage = 0
			s.StatusMsgID = msgID
		})
		b.removeKb(chatID, msgID)
		b.askMode(chatID, sess)
	case cbPlSelect:
		sess.mutate(func(s *Session) {
			s.State = StateAwaitingPlaylistSelection
			s.StatusMsgID = msgID
			s.SelectedEntries = nil
			s.SelectedIndices = make(map[int]bool)
			s.PlaylistPage = 0
		})
		b.editKb(chatID, msgID, b.playlistSelectionText(sess), kbPlaylistSelection(sess))
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

	selected := make(map[int]bool, len(indices))
	for _, idx := range indices {
		selected[idx] = true
	}
	entries := playlistEntriesFromSelection(snap.PlInfo, selected)
	sess.mutate(func(s *Session) {
		s.SelectedIndices = selected
		s.SelectedEntries = entries
		s.QualityChoices = nil
		s.Profile = app.OutputProfile{}
	})
	b.askMode(chatID, sess)
}

func (b *Bot) handlePlaylistSelectionCallback(chatID int64, msgID int, sess *Session, data string) string {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return "Сессия устарела. Пришли ссылку заново."
	}

	selected := cloneSelection(snap.SelectedIndices)
	if selected == nil {
		selected = make(map[int]bool)
	}

	switch {
	case data == cbPlSelectAll:
		selected = make(map[int]bool, len(snap.PlInfo.Entries))
		for _, entry := range snap.PlInfo.Entries {
			selected[entry.Index] = true
		}
	case data == cbPlSelectNone:
		selected = make(map[int]bool)
	case data == cbPlSelectDone:
		entries := playlistEntriesFromSelection(snap.PlInfo, selected)
		if len(entries) == 0 {
			return "Выбери хотя бы одно видео."
		}
		sess.mutate(func(s *Session) {
			s.SelectedIndices = selected
			s.SelectedEntries = entries
			s.QualityChoices = nil
			s.Profile = app.OutputProfile{}
			s.StatusMsgID = msgID
		})
		b.askMode(chatID, sess)
		return ""
	case strings.HasPrefix(data, cbPlTogglePref):
		idx, err := strconv.Atoi(strings.TrimPrefix(data, cbPlTogglePref))
		if err != nil {
			return ""
		}
		if selected[idx] {
			delete(selected, idx)
		} else if idx >= 1 && idx <= len(snap.PlInfo.Entries) {
			selected[idx] = true
		}
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

	sess.mutate(func(s *Session) {
		s.SelectedIndices = selected
		s.SelectedEntries = nil
	})
	b.editKb(chatID, msgID, b.playlistSelectionText(sess), kbPlaylistSelection(sess))
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
		return err.Error()
	}
	return ""
}

func (b *Bot) runDownload(chatID int64, sess *Session, req app.DownloadRequest) {
	snap := sess.snapshot()
	msgID := snap.StatusMsgID

	dlCh := make(chan app.DlUpdate, 256)
	entries := b.playlistEntriesForSession(sess)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		stopCh := sess.stopSignal()
		if stopCh == nil {
			return
		}
		<-stopCh
		cancel()
	}()

	app.StartDownloadRequestContext(ctx, req, dlCh)

	var (
		lastEdit  time.Time
		totalDone int
		totalFail int
		total     = len(entries)
		dlStem    string
		cancelled bool
	)

	editThrottle := 2 * time.Second

	for u := range dlCh {
		if sess.isCancelled() {
			cancelled = true
			continue
		}

		switch u.Type {
		case app.EvDest:
			dlStem = u.Text

		case app.EvProgress:
			if time.Since(lastEdit) < editThrottle {
				continue
			}
			lastEdit = time.Now()
			if total > 0 {
				done := totalDone + totalFail
				b.edit(chatID, msgID, fmt.Sprintf(
					"⬇️ Плейлист: %s\n%d / %d  (%.1f%%)\n%s  %s",
					progressBar(done, total), done, total,
					float64(done)/float64(total)*100,
					app.FmtBytesFor(u.DoneB, app.LocaleRU), u.Speed,
				))
			} else {
				b.edit(chatID, msgID, fmt.Sprintf(
					"⬇️ <b>%s</b>\n%.1f%%  %s  %s",
					escapeHTML(dlStem), u.Pct,
					app.FmtBytesFor(u.DoneB, app.LocaleRU), u.Speed,
				))
			}

		case app.EvProc:
			if time.Since(lastEdit) < editThrottle {
				continue
			}
			lastEdit = time.Now()
			b.edit(chatID, msgID, "⚙️ "+escapeHTML(u.Text))

		case app.EvDone:
			if u.OK {
				totalDone++
			} else {
				totalFail++
			}
			if total > 0 && totalDone+totalFail < total {
				continue
			}
			b.onDownloadFinished(chatID, msgID, sess, dlStem, totalDone, totalFail, total)
			b.sessions.reset(chatID)
			return
		}
	}

	if cancelled || sess.isCancelled() {
		cleanupBotWorkDir(snap.WorkDir)
		b.edit(chatID, msgID, "❌ Скачивание отменено.")
		b.sessions.reset(chatID)
	}
}

func (b *Bot) onDownloadFinished(
	chatID int64, msgID int, sess *Session,
	dlStem string,
	done, failed, total int,
) {
	snap := sess.snapshot()
	newFiles := filesInDir(snap.WorkDir)
	defer cleanupBotWorkDir(snap.WorkDir)

	if total == 0 {
		if failed > 0 || done == 0 {
			if len(newFiles) > 0 {
				b.edit(chatID, msgID, "❌ Не удалось скачать видео. Временные файлы удалены.")
			} else {
				b.edit(chatID, msgID, "❌ Не удалось скачать видео.")
			}
			return
		}
		if len(newFiles) == 0 {
			b.edit(chatID, msgID, "✅ Скачано!")
			return
		}
		b.edit(chatID, msgID, "✅ Готово! Отправляю файл…")
		sent, tooLarge, sendErr := b.sendDownloadedFiles(chatID, newFiles)
		b.edit(chatID, msgID, b.singleSendSummary(sent, tooLarge, sendErr))
		return
	}

	icon := "✅"
	if failed > 0 && done == 0 {
		icon = "❌"
	} else if failed > 0 {
		icon = "⚠️"
	}

	if len(newFiles) == 0 {
		b.edit(chatID, msgID, fmt.Sprintf(
			"%s <b>Плейлист завершён</b>\n\n✔ Успешно: %d\n✘ Ошибок: %d\nИтого: %d",
			icon, done, failed, total,
		))
		return
	}

	b.edit(chatID, msgID, fmt.Sprintf(
		"%s <b>Плейлист завершён</b>\n\n✔ Успешно: %d\n✘ Ошибок: %d\nИтого: %d\n\n📤 Отправляю файлы в Telegram…",
		icon, done, failed, total,
	))
	sent, tooLarge, sendErr := b.sendDownloadedFiles(chatID, newFiles)
	b.edit(chatID, msgID, b.playlistSendSummary(icon, done, failed, total, sent, tooLarge, sendErr))
}

func (b *Bot) qualityScanText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		count := len(b.playlistEntriesForSession(sess))
		if !app.ShouldScanQualityChoices(count) {
			return fmt.Sprintf("⚡ Готовлю быстрый выбор качества для %d видео…", count)
		}
		return fmt.Sprintf("🔍 Сканирую качества и размер для %d видео…", count)
	}
	return "🔍 Сканирую качества и размер…"
}

func (b *Bot) qualityPromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf(
			"📋 <b>%s</b>\n%d видео\n\nВыбери качество видео:",
			escapeHTML(snap.PlInfo.Title),
			len(b.playlistEntriesForSession(sess)),
		)
	}
	return "🎬 Выбери качество видео:"
}

func (b *Bot) qualityScanURLs(sess *Session) []string {
	entries := b.playlistEntriesForSession(sess)
	snap := sess.snapshot()
	if snap.PlInfo == nil || snap.ForceSingle || len(entries) == 0 {
		return []string{snap.Target.DownloadURL(snap.ForceSingle)}
	}

	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}
	return urls
}

func (b *Bot) playlistScopeText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return "📋 Выбери, что скачать:"
	}
	return fmt.Sprintf(
		"📋 <b>%s</b>\n%d видео\n\nЧто скачать?",
		escapeHTML(snap.PlInfo.Title),
		len(snap.PlInfo.Entries),
	)
}

func (b *Bot) playlistEntriesForSession(sess *Session) []app.PlaylistEntry {
	snap := sess.snapshot()
	if snap.PlInfo == nil || snap.ForceSingle {
		return nil
	}
	if len(snap.SelectedEntries) > 0 {
		return snap.SelectedEntries
	}
	if len(snap.SelectedIndices) > 0 {
		return playlistEntriesFromSelection(snap.PlInfo, snap.SelectedIndices)
	}
	return snap.PlInfo.Entries
}

func (b *Bot) modePromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf("📋 <b>%s</b>\n%d видео\n\nЧто скачать?", escapeHTML(snap.PlInfo.Title), len(b.playlistEntriesForSession(sess)))
	}
	return "🎛 <b>Выбери режим</b>\n\nЧто скачать?"
}

func (b *Bot) audioPromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf("📋 <b>%s</b>\n%d видео\n\nВыбери аудио:", escapeHTML(snap.PlInfo.Title), len(b.playlistEntriesForSession(sess)))
	}
	return "🎵 Выбери аудио:"
}

func (b *Bot) buildDownloadRequest(sess *Session) (app.DownloadRequest, error) {
	snap := sess.snapshot()
	profile := snap.Profile
	if profile.Mode == 0 {
		profile = app.DefaultProfileForMode(snap.Mode, app.LocaleRU)
	}

	req := app.NormalizeDownloadRequest(app.DownloadRequest{
		Target:       snap.Target,
		Profile:      profile,
		ForceSingle:  snap.ForceSingle,
		PlaylistInfo: snap.PlInfo,
		Entries:      b.playlistEntriesForSession(sess),
		Workers:      app.AutoDownloadWorkers(len(b.playlistEntriesForSession(sess))),
		OutputDir:    snap.WorkDir,
		Locale:       app.LocaleRU,
	})
	if err := app.ValidateDownloadRequest(req); err != nil {
		return app.DownloadRequest{}, err
	}
	return req, nil
}

func (b *Bot) downloadStartText(sess *Session) string {
	snap := sess.snapshot()
	profile := snap.Profile
	if profile.Mode == 0 {
		profile = app.DefaultProfileForMode(snap.Mode, app.LocaleRU)
	}
	modeLabel := profile.Label
	if modeLabel == "" {
		switch profile.Mode {
		case app.ModeAudio:
			modeLabel = "Аудио"
		case app.ModeThumbnail:
			modeLabel = "Превью"
		default:
			modeLabel = "Видео"
		}
	}

	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf("📋 <b>%s</b>\n%d видео\n🎛 Режим: %s\n\n⬇️ Начинаю скачивание…",
			escapeHTML(snap.PlInfo.Title),
			len(b.playlistEntriesForSession(sess)),
			escapeHTML(modeLabel),
		)
	}
	return fmt.Sprintf("🎬 %s\n\n⬇️ Начинаю скачивание…", escapeHTML(modeLabel))
}
