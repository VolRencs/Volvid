package bot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	if !messageIsCommand(msg) && sess != nil && sess.State == StateAwaitingPlaylistSelection && text != "" && !app.YtRE.MatchString(text) {
		b.handlePlaylistSelectionInput(chatID, sess, text)
		return
	}

	switch {
	case messageIsCommand(msg):
		b.handleCommand(msg)
	case app.YtRE.MatchString(text):
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
		if sess == nil || sess.State == StateIdle {
			b.send(chatID, "Нечего отменять.")
			return
		}
		sess.cancel()
		b.sessions.set(chatID, &Session{State: StateIdle})
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

func (b *Bot) handleURL(chatID int64, url string) {
	sess := b.sessions.get(chatID)
	if sess != nil && sess.State == StateDownloading {
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

	newSess := &Session{State: StateIdle, URL: url, WorkDir: workDir, stopCh: make(chan struct{})}
	b.sessions.set(chatID, newSess)

	if app.VideoInPlaylistRE.MatchString(url) {
		newSess.State = StateAwaitingPlaylistOp
		b.sendKb(chatID, "⚠️ Ссылка содержит и видео, и плейлист. Что скачать?", kbPlaylistChoice())
		return
	}

	if app.IsPlaylistURL(url) {
		b.fetchAndAskQuality(chatID, newSess)
		return
	}

	b.scanAndAskQuality(chatID, newSess)
}

func (b *Bot) fetchAndAskQuality(chatID int64, sess *Session) {
	sess.State = StateFetchingPlaylist
	statusMsg, _ := b.send(chatID, "⏳ Загружаю список плейлиста…")
	sess.StatusMsgID = statusMsg.ID

	go func() {
		info, err := app.FetchPlaylistInfo(sess.URL)
		if sess.isCancelled() {
			return
		}
		if err != nil || info == nil {
			sess.ForceSingle = true
			sess.SelectedEntries = nil
			b.scanAndAskQuality(chatID, sess)
			return
		}
		sess.PlInfo = info
		sess.SelectedEntries = nil
		sess.SelectedIndices = nil
		sess.PlaylistPage = 0
		sess.QualityChoices = nil
		sess.State = StateAwaitingPlaylistScope
		b.editKb(chatID, sess.StatusMsgID, b.playlistScopeText(sess), kbPlaylistScope())
	}()
}

func (b *Bot) scanAndAskQuality(chatID int64, sess *Session) {
	sess.State = StateFetchingQuality
	if sess.StatusMsgID == 0 {
		statusMsg, _ := b.send(chatID, b.qualityScanText(sess))
		sess.StatusMsgID = statusMsg.ID
	} else {
		b.edit(chatID, sess.StatusMsgID, b.qualityScanText(sess))
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
		sess.QualityChoices = choices
		sess.State = StateAwaitingQuality
		b.editKb(chatID, sess.StatusMsgID, b.qualityPromptText(sess), kbQuality(choices))
	}()
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
		b.sessions.set(chatID, &Session{State: StateIdle})
		b.edit(chatID, msgID, "❌ Отменено.")
		return
	}
	if data == cbNoop {
		b.answer(cq, "")
		return
	}

	var alert string
	switch sess.State {
	case StateAwaitingPlaylistOp:
		alert = b.handlePlaylistOpCallback(chatID, msgID, sess, data)
	case StateAwaitingPlaylistScope:
		alert = b.handlePlaylistScopeCallback(chatID, msgID, sess, data)
	case StateAwaitingPlaylistSelection:
		alert = b.handlePlaylistSelectionCallback(chatID, msgID, sess, data)
	case StateAwaitingQuality:
		alert = b.handleQualityCallback(chatID, msgID, sess, data)
	}
	b.answer(cq, alert)
}

func (b *Bot) handlePlaylistOpCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbPlVideo:
		sess.ForceSingle = true
		sess.StatusMsgID = msgID
		b.removeKb(chatID, msgID)
		b.scanAndAskQuality(chatID, sess)
	case cbPlFull:
		sess.StatusMsgID = msgID
		b.removeKb(chatID, msgID)
		b.fetchAndAskQuality(chatID, sess)
	}
	return ""
}

func (b *Bot) handlePlaylistScopeCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbPlAll:
		sess.SelectedEntries = append([]app.PlaylistEntry(nil), sess.PlInfo.Entries...)
		sess.SelectedIndices = nil
		sess.PlaylistPage = 0
		sess.StatusMsgID = msgID
		b.removeKb(chatID, msgID)
		b.scanAndAskQuality(chatID, sess)
	case cbPlSelect:
		sess.State = StateAwaitingPlaylistSelection
		sess.StatusMsgID = msgID
		sess.SelectedEntries = nil
		sess.SelectedIndices = make(map[int]bool)
		sess.PlaylistPage = 0
		b.editKb(chatID, msgID, b.playlistSelectionText(sess), kbPlaylistSelection(sess))
	}
	return ""
}

func (b *Bot) handlePlaylistSelectionInput(chatID int64, sess *Session, raw string) {
	if sess.PlInfo == nil {
		b.send(chatID, "⚠️ Сессия устарела. Пришли ссылку заново.")
		b.sessions.set(chatID, &Session{State: StateIdle})
		return
	}

	indices, err := app.ParseSelection(raw, len(sess.PlInfo.Entries))
	if err != nil {
		b.send(chatID, "⚠️ "+escapeHTML(err.Error())+"\n\nПример: <code>1-3,7,10</code> или <code>all</code>")
		return
	}

	sess.SelectedIndices = make(map[int]bool, len(indices))
	for _, idx := range indices {
		sess.SelectedIndices[idx] = true
	}
	sess.SelectedEntries = playlistEntriesFromSelection(sess.PlInfo, sess.SelectedIndices)
	sess.QualityChoices = nil
	b.scanAndAskQuality(chatID, sess)
}

func (b *Bot) handlePlaylistSelectionCallback(chatID int64, msgID int, sess *Session, data string) string {
	if sess.PlInfo == nil {
		return "Сессия устарела. Пришли ссылку заново."
	}
	if sess.SelectedIndices == nil {
		sess.SelectedIndices = make(map[int]bool)
	}

	switch {
	case data == cbPlSelectAll:
		sess.SelectedIndices = make(map[int]bool, len(sess.PlInfo.Entries))
		for _, entry := range sess.PlInfo.Entries {
			sess.SelectedIndices[entry.Index] = true
		}
	case data == cbPlSelectNone:
		sess.SelectedIndices = make(map[int]bool)
	case data == cbPlSelectDone:
		sess.SelectedEntries = playlistEntriesFromSelection(sess.PlInfo, sess.SelectedIndices)
		if len(sess.SelectedEntries) == 0 {
			return "Выбери хотя бы одно видео."
		}
		sess.QualityChoices = nil
		sess.StatusMsgID = msgID
		b.scanAndAskQuality(chatID, sess)
		return ""
	case strings.HasPrefix(data, cbPlTogglePref):
		idx, err := strconv.Atoi(strings.TrimPrefix(data, cbPlTogglePref))
		if err != nil {
			return ""
		}
		if sess.SelectedIndices[idx] {
			delete(sess.SelectedIndices, idx)
		} else if idx >= 1 && idx <= len(sess.PlInfo.Entries) {
			sess.SelectedIndices[idx] = true
		}
	case strings.HasPrefix(data, cbPlPagePref):
		page, err := strconv.Atoi(strings.TrimPrefix(data, cbPlPagePref))
		if err != nil {
			return ""
		}
		sess.PlaylistPage = page
	default:
		return ""
	}

	sess.SelectedEntries = nil
	b.editKb(chatID, msgID, b.playlistSelectionText(sess), kbPlaylistSelection(sess))
	return ""
}

func (b *Bot) handleQualityCallback(chatID int64, msgID int, sess *Session, data string) string {
	if !strings.HasPrefix(data, cbQualityPrefix) {
		return ""
	}
	choices := sess.QualityChoices
	if len(choices) == 0 {
		choices = app.DefaultQualityChoices()
	}
	choice, ok := app.FindQualityChoice(choices, strings.TrimPrefix(data, cbQualityPrefix))
	if !ok {
		return ""
	}
	cfg := choice.Config(app.LocaleRU)

	sess.State = StateDownloading
	sess.StatusMsgID = msgID

	b.removeKb(chatID, msgID)

	var sizeNote string
	if choice.SizeBytes > 0 {
		sizeStr := app.FmtBytesFor(choice.SizeBytes, app.LocaleRU)
		aboveLimit := choice.SizeBytes > b.sendLimitBytes()
		if aboveLimit {
			sizeNote = fmt.Sprintf("\n📦 Размер: ~%s\n⚠️ %s не принимает файлы больше %s", sizeStr, b.backendLabel(), b.sendLimitText())
		} else {
			sizeNote = fmt.Sprintf("\n📦 Размер: ~%s", sizeStr)
		}
	}

	var desc string
	if sess.PlInfo != nil && !sess.ForceSingle {
		desc = fmt.Sprintf("📋 <b>%s</b>\n%d видео%s\n\n⬇️ Начинаю скачивание…",
			escapeHTML(sess.PlInfo.Title), len(b.playlistEntriesForSession(sess)), sizeNote)
	} else {
		desc = "🎬 Видео" + sizeNote + "\n\n⬇️ Начинаю скачивание…"
	}
	b.edit(chatID, msgID, desc)

	go b.runDownload(chatID, sess, cfg)
	return ""
}

func (b *Bot) runDownload(chatID int64, sess *Session, cfg app.QualityConfig) {
	msgID := sess.StatusMsgID

	dlCh := make(chan app.DlUpdate, 256)
	entries := b.playlistEntriesForSession(sess)
	workers := app.AutoDownloadWorkers(len(entries))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if sess.stopCh == nil {
			return
		}
		<-sess.stopCh
		cancel()
	}()

	app.StartDownloadContextInDir(ctx, sess.WorkDir, cfg, sess.URL, sess.ForceSingle, sess.PlInfo, entries, workers, dlCh)

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
			b.sessions.set(chatID, &Session{State: StateIdle})
			return
		}
	}

	if cancelled || sess.isCancelled() {
		cleanupBotWorkDir(sess.WorkDir)
		b.edit(chatID, msgID, "❌ Скачивание отменено.")
		b.sessions.set(chatID, &Session{State: StateIdle})
	}
}

func (b *Bot) onDownloadFinished(
	chatID int64, msgID int, sess *Session,
	dlStem string,
	done, failed, total int,
) {
	newFiles := filesInDir(sess.WorkDir)
	defer cleanupBotWorkDir(sess.WorkDir)

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
	if sess.PlInfo != nil && !sess.ForceSingle {
		count := len(b.playlistEntriesForSession(sess))
		if !app.ShouldScanQualityChoices(count) {
			return fmt.Sprintf("⚡ Готовлю быстрый выбор качества для %d видео…", count)
		}
		return fmt.Sprintf("🔍 Сканирую качества и размер для %d видео…", count)
	}
	return "🔍 Сканирую качества и размер…"
}

func (b *Bot) qualityPromptText(sess *Session) string {
	if sess.PlInfo != nil && !sess.ForceSingle {
		return fmt.Sprintf(
			"📋 <b>%s</b>\n%d видео\n\nВыбери качество:",
			escapeHTML(sess.PlInfo.Title),
			len(b.playlistEntriesForSession(sess)),
		)
	}
	return "🎬 Выбери качество:"
}

func (b *Bot) qualityScanURLs(sess *Session) []string {
	entries := b.playlistEntriesForSession(sess)
	if sess.PlInfo == nil || sess.ForceSingle || len(entries) == 0 {
		return []string{sess.URL}
	}

	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}
	return urls
}

func (b *Bot) playlistScopeText(sess *Session) string {
	if sess.PlInfo == nil {
		return "📋 Выбери, что скачать:"
	}
	return fmt.Sprintf(
		"📋 <b>%s</b>\n%d видео\n\nЧто скачать?",
		escapeHTML(sess.PlInfo.Title),
		len(sess.PlInfo.Entries),
	)
}

func (b *Bot) playlistEntriesForSession(sess *Session) []app.PlaylistEntry {
	if sess == nil || sess.PlInfo == nil || sess.ForceSingle {
		return nil
	}
	if len(sess.SelectedEntries) > 0 {
		return sess.SelectedEntries
	}
	if len(sess.SelectedIndices) > 0 {
		return playlistEntriesFromSelection(sess.PlInfo, sess.SelectedIndices)
	}
	return sess.PlInfo.Entries
}

func filesInDir(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out
}

func (b *Bot) sendDownloadedFiles(chatID int64, paths []string) (sent, tooLarge, sendErr int) {
	for _, path := range paths {
		err := b.sendFile(chatID, path)
		switch {
		case err == nil:
			sent++
		case errors.Is(err, errTelegramFileTooLarge):
			tooLarge++
		default:
			sendErr++
		}
	}
	return sent, tooLarge, sendErr
}

func (b *Bot) singleSendSummary(sent, tooLarge, sendErr int) string {
	switch {
	case sent > 0 && tooLarge == 0 && sendErr == 0:
		return "✅ Готово! Файл удалён с сервера."
	case sent > 0:
		return fmt.Sprintf(
			"✅ Скачано!\n\n📤 Отправлено: %d\n📦 Слишком большие для %s: %d\n⚠️ Не удалось отправить: %d\n\n%s\nФайлы удалены с сервера.",
			sent, b.backendLabel(), tooLarge, sendErr,
			b.sendLimitNotice(),
		)
	case tooLarge > 0:
		return fmt.Sprintf("✅ Скачано!\n\nФайл слишком большой для %s.\n\n%s\nФайл удалён с сервера.", b.backendLabel(), b.sendLimitNotice())
	default:
		return "✅ Скачано!\n\nНе удалось отправить файл в Telegram.\nФайл удалён с сервера."
	}
}

func (b *Bot) playlistSendSummary(icon string, done, failed, total, sent, tooLarge, sendErr int) string {
	var extra string
	switch {
	case sent > 0 || tooLarge > 0 || sendErr > 0:
		extra = fmt.Sprintf(
			"\n\n📤 Отправлено: %d\n📦 Слишком большие для %s: %d\n⚠️ Не удалось отправить: %d",
			sent, b.backendLabel(), tooLarge, sendErr,
		)
		if tooLarge > 0 {
			extra += "\n\n" + b.sendLimitNotice()
		}
		extra += "\nВсе файлы удалены с сервера."
	}

	return fmt.Sprintf(
		"%s <b>Плейлист завершён</b>\n\n✔ Успешно: %d\n✘ Ошибок: %d\nИтого: %d%s",
		icon, done, failed, total, extra,
	)
}

func cleanupBotWorkDir(dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	parent := filepath.Dir(dir)
	if parent != "" && parent != "." && parent != app.DlDir {
		_ = os.Remove(parent)
	}
}

func progressBar(done, total int) string {
	const width = 10
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := done * width / total
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}

func verLine(name, ver string) string {
	if ver == "" {
		return fmt.Sprintf("• %s — <b>не найден</b>", escapeHTML(name))
	}
	return fmt.Sprintf("• %s — <code>%s</code>", escapeHTML(name), escapeHTML(ver))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func messageIsCommand(msg *models.Message) bool {
	if msg == nil {
		return false
	}
	for _, entity := range msg.Entities {
		if entity.Type == models.MessageEntityTypeBotCommand && entity.Offset == 0 {
			return true
		}
	}
	return false
}

func messageCommand(msg *models.Message) string {
	if msg == nil {
		return ""
	}
	for _, entity := range msg.Entities {
		if entity.Type != models.MessageEntityTypeBotCommand || entity.Offset != 0 || entity.Length <= 1 {
			continue
		}
		runes := []rune(msg.Text)
		if entity.Offset+entity.Length > len(runes) {
			return ""
		}
		cmd := strings.TrimPrefix(string(runes[entity.Offset:entity.Offset+entity.Length]), "/")
		cmd, _, _ = strings.Cut(cmd, "@")
		return cmd
	}
	return ""
}

func callbackMessageMeta(cq *models.CallbackQuery) (chatID int64, msgID int, ok bool) {
	if cq == nil {
		return 0, 0, false
	}
	switch cq.Message.Type {
	case models.MaybeInaccessibleMessageTypeMessage:
		if cq.Message.Message == nil {
			return 0, 0, false
		}
		return cq.Message.Message.Chat.ID, cq.Message.Message.ID, true
	case models.MaybeInaccessibleMessageTypeInaccessibleMessage:
		if cq.Message.InaccessibleMessage == nil {
			return 0, 0, false
		}
		return cq.Message.InaccessibleMessage.Chat.ID, cq.Message.InaccessibleMessage.MessageID, true
	default:
		return 0, 0, false
	}
}
