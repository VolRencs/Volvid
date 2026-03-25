package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) handleURL(chatID, userID int64, rawURL string) {
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

	workDir, err := createBotWorkDir(userID)
	if err != nil {
		b.send(chatID, "⚠️ Не удалось подготовить временную папку для загрузки.")
		return
	}

	newSess := newSession(userID, rawURL, workDir)
	newSess.mutate(func(s *Session) {
		s.Target = target
	})
	b.sessions.set(chatID, newSess)

	switch {
	case target.Kind == app.TargetMixed:
		newSess.mutate(func(s *Session) {
			s.State = StateAwaitingPlaylistOp
		})
		b.sendKb(chatID, "⚠️ Ссылка содержит и видео, и плейлист. Что скачать?", kbPlaylistChoice())
	case target.IsPlaylist():
		b.fetchAndAskPlaylist(chatID, newSess)
	default:
		b.askMode(chatID, newSess)
	}
}

func (b *Bot) fetchAndAskPlaylist(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateFetchingPlaylist
		s.ForceSingle = false
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
		if err != nil || info == nil || len(info.Entries) == 0 {
			b.failPlaylistSelection(chatID, sess)
			return
		}
		sess.mutate(func(s *Session) {
			s.PlInfo = info
			s.SelectedIndices = nil
			s.PlaylistPage = 0
			s.QualityChoices = nil
			s.Profile = app.OutputProfile{}
		})
		state := sess.snapshot()
		b.openPlaylistSelection(chatID, state.StatusMsgID, sess)
	}()
}

func (b *Bot) failPlaylistSelection(chatID int64, sess *Session) {
	snap := sess.snapshot()
	text := "⚠️ Не удалось загрузить список плейлиста. Пришли ссылку заново."
	if snap.StatusMsgID != 0 {
		b.edit(chatID, snap.StatusMsgID, text)
	} else {
		b.send(chatID, text)
	}
	b.sessions.reset(chatID)
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
		var choices []app.QualityChoice
		if len(urls) > 0 {
			choices, _ = app.ResolveQualityChoices(urls)
		}
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
		sess.mutate(func(s *Session) {
			s.StatusMsgID = msg.ID
		})
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
	b.editKb(chatID, sess.snapshot().StatusMsgID, b.audioPromptText(sess), kbAudioProfiles(profiles))
}

func (b *Bot) startConfiguredDownload(chatID int64, sess *Session) error {
	req, err := b.buildDownloadRequest(sess)
	if err != nil {
		return err
	}

	snap := sess.snapshot()
	estimate, err := app.EstimateDownloadSize(req)
	if err == nil && estimate.TotalBytes > b.downloadSizeLimit(snap.UserID) {
		b.notifyDownloadSizeLimitExceeded(chatID, snap.StatusMsgID, snap.UserID)
		return errDownloadLimitExceeded
	}

	sess.mutate(func(s *Session) {
		s.State = StateDownloading
	})

	snap = sess.snapshot()
	b.removeKb(chatID, snap.StatusMsgID)
	b.edit(chatID, snap.StatusMsgID, b.downloadStartText(sess))
	go b.runDownload(chatID, sess, req)
	return nil
}

func (b *Bot) runDownload(chatID int64, sess *Session, req app.DownloadRequest) {
	snap := sess.snapshot()
	msgID := snap.StatusMsgID

	dlCh := make(chan app.DlUpdate, 256)
	entries := b.playlistSelectionEntries(sess)
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

	const editThrottle = 2 * time.Second

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
			b.edit(chatID, msgID, b.downloadProgressText(totalDone, totalFail, total, dlStem, u))
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
			b.onDownloadFinished(chatID, msgID, sess, doneSummary{Stem: dlStem, OK: totalDone, Failed: totalFail, Total: total})
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

type doneSummary struct {
	Stem   string
	OK     int
	Failed int
	Total  int
}

func (b *Bot) onDownloadFinished(chatID int64, msgID int, sess *Session, summary doneSummary) {
	snap := sess.snapshot()
	actualSize, sizeErr := app.DirSize(snap.WorkDir)
	newFiles := filesInDir(snap.WorkDir)
	defer cleanupBotWorkDir(snap.WorkDir)

	if sizeErr == nil && actualSize > b.downloadSizeLimit(snap.UserID) {
		b.notifyDownloadSizeLimitExceeded(chatID, msgID, snap.UserID)
		return
	}

	if summary.Total == 0 {
		b.finishSingleDownload(chatID, msgID, newFiles, summary.Failed > 0 || summary.OK == 0)
		return
	}
	b.finishPlaylistDownload(chatID, msgID, newFiles, summary)
}

func (b *Bot) finishSingleDownload(chatID int64, msgID int, newFiles []string, failed bool) {
	if failed {
		if len(newFiles) > 0 {
			b.edit(chatID, msgID, "❌ Не удалось скачать видео. Временные файлы удалены.")
			return
		}
		b.edit(chatID, msgID, "❌ Не удалось скачать видео.")
		return
	}
	if len(newFiles) == 0 {
		b.edit(chatID, msgID, "✅ Скачано!")
		return
	}

	b.edit(chatID, msgID, "✅ Готово! Отправляю файл…")
	sent, tooLarge, sendErr := b.sendDownloadedFiles(chatID, newFiles)
	b.edit(chatID, msgID, b.singleSendSummary(sent, tooLarge, sendErr))
}

func (b *Bot) finishPlaylistDownload(chatID int64, msgID int, newFiles []string, summary doneSummary) {
	icon := playlistResultIcon(summary.OK, summary.Failed)
	if len(newFiles) == 0 {
		b.edit(chatID, msgID, fmt.Sprintf(
			"%s <b>Плейлист завершён</b>\n\n✔ Успешно: %d\n✘ Ошибок: %d\nИтого: %d",
			icon, summary.OK, summary.Failed, summary.Total,
		))
		return
	}

	b.edit(chatID, msgID, fmt.Sprintf(
		"%s <b>Плейлист завершён</b>\n\n✔ Успешно: %d\n✘ Ошибок: %d\nИтого: %d\n\n📤 Отправляю файлы в Telegram…",
		icon, summary.OK, summary.Failed, summary.Total,
	))
	sent, tooLarge, sendErr := b.sendDownloadedFiles(chatID, newFiles)
	b.edit(chatID, msgID, b.playlistSendSummary(icon, summary.OK, summary.Failed, summary.Total, sent, tooLarge, sendErr))
}

func playlistResultIcon(done, failed int) string {
	switch {
	case failed > 0 && done == 0:
		return "❌"
	case failed > 0:
		return "⚠️"
	default:
		return "✅"
	}
}

func (b *Bot) buildDownloadRequest(sess *Session) (app.DownloadRequest, error) {
	snap := sess.snapshot()
	profile := snap.Profile
	if profile.Mode == 0 {
		profile = app.DefaultProfileForMode(snap.Mode, app.LocaleRU)
	}
	entries := b.playlistSelectionEntries(sess)
	if snap.PlInfo != nil && !snap.ForceSingle {
		if len(entries) == 0 {
			return app.DownloadRequest{}, errors.New("Сначала выбери хотя бы одно видео из плейлиста.")
		}
		if !b.validatePlaylistSelectionCount(snap.UserID, len(entries)) {
			return app.DownloadRequest{}, errors.New(b.playlistItemLimitAlert(snap.UserID))
		}
	}

	req := app.NormalizeDownloadRequest(app.DownloadRequest{
		Target:       snap.Target,
		Profile:      profile,
		ForceSingle:  snap.ForceSingle,
		PlaylistInfo: snap.PlInfo,
		Entries:      entries,
		Workers:      app.AutoDownloadWorkers(len(entries)),
		OutputDir:    snap.WorkDir,
		Locale:       app.LocaleRU,
	})
	if err := app.ValidateDownloadRequest(req); err != nil {
		return app.DownloadRequest{}, err
	}
	return req, nil
}

func (b *Bot) downloadProgressText(done, failed, total int, dlStem string, update app.DlUpdate) string {
	if total > 0 {
		completed := done + failed
		return fmt.Sprintf(
			"⬇️ Плейлист: %s\n%d / %d  (%.1f%%)\n%s  %s",
			progressBar(completed, total), completed, total,
			float64(completed)/float64(total)*100,
			app.FmtBytesFor(update.DoneB, app.LocaleRU), update.Speed,
		)
	}
	return fmt.Sprintf(
		"⬇️ <b>%s</b>\n%.1f%%  %s  %s",
		escapeHTML(dlStem), update.Pct,
		app.FmtBytesFor(update.DoneB, app.LocaleRU), update.Speed,
	)
}

func (b *Bot) qualityScanText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		count := len(b.playlistSelectionEntries(sess))
		if count == 0 {
			return "🔍 Сканирую качества и размер…"
		}
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
			len(b.playlistSelectionEntries(sess)),
		)
	}
	return "🎬 Выбери качество видео:"
}

func (b *Bot) qualityScanURLs(sess *Session) []string {
	entries := b.playlistSelectionEntries(sess)
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle && len(entries) == 0 {
		return nil
	}
	if snap.PlInfo == nil || snap.ForceSingle {
		return []string{snap.Target.DownloadURL(snap.ForceSingle)}
	}

	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}
	return urls
}

func (b *Bot) modePromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf("📋 <b>%s</b>\n%d видео\n\nЧто скачать?", escapeHTML(snap.PlInfo.Title), len(b.playlistSelectionEntries(sess)))
	}
	return "🎛 <b>Выбери режим</b>\n\nЧто скачать?"
}

func (b *Bot) audioPromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf("📋 <b>%s</b>\n%d видео\n\nВыбери аудио:", escapeHTML(snap.PlInfo.Title), len(b.playlistSelectionEntries(sess)))
	}
	return "🎵 Выбери аудио:"
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
		return fmt.Sprintf(
			"📋 <b>%s</b>\n%d видео\n🎛 Режим: %s\n\n⬇️ Начинаю скачивание…",
			escapeHTML(snap.PlInfo.Title),
			len(b.playlistSelectionEntries(sess)),
			escapeHTML(modeLabel),
		)
	}
	return fmt.Sprintf("🎬 %s\n\n⬇️ Начинаю скачивание…", escapeHTML(modeLabel))
}
