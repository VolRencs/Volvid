package bot

import (
	"context"
	"fmt"
	"time"

	app "YouTubeBuild/internal/app"
)

const downloadEditThrottle = 2 * time.Second

type doneSummary struct {
	Stem   string
	OK     int
	Failed int
	Total  int
}

type progressEditor struct {
	lastEdit time.Time
}

func (e *progressEditor) Allow(now time.Time) bool {
	if now.Sub(e.lastEdit) < downloadEditThrottle {
		return false
	}
	e.lastEdit = now
	return true
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

	state := doneSummary{Total: len(entries)}
	editor := &progressEditor{}
	cancelled := false

	for u := range dlCh {
		if sess.isCancelled() {
			cancelled = true
			continue
		}

		finished := b.handleDownloadUpdate(chatID, msgID, sess, u, &state, editor)
		if !finished {
			continue
		}
		b.sessions.reset(chatID)
		return
	}

	if cancelled || sess.isCancelled() {
		b.logf("download cancelled %s", logChatUser(chatID, snap.UserID))
		cleanupBotWorkDir(snap.WorkDir)
		b.edit(chatID, msgID, "❌ Скачивание отменено.")
		b.sessions.reset(chatID)
	}
}

func (b *Bot) handleDownloadUpdate(
	chatID int64,
	msgID int,
	sess *Session,
	update app.DlUpdate,
	state *doneSummary,
	editor *progressEditor,
) bool {
	switch update.Type {
	case app.EvDest:
		state.Stem = update.Text
	case app.EvProgress:
		if editor.Allow(time.Now()) {
			b.edit(chatID, msgID, b.downloadProgressText(state.OK, state.Failed, state.Total, state.Stem, update))
		}
	case app.EvProc:
		if editor.Allow(time.Now()) {
			b.edit(chatID, msgID, "⚙️ "+escapeHTML(update.Text))
		}
	case app.EvDone:
		if update.OK {
			state.OK++
		} else {
			state.Failed++
		}
		if state.Total > 0 && state.OK+state.Failed < state.Total {
			return false
		}
		b.onDownloadFinished(chatID, msgID, sess, *state)
		return true
	}
	return false
}

func (b *Bot) onDownloadFinished(chatID int64, msgID int, sess *Session, summary doneSummary) {
	snap := sess.snapshot()
	actualSize, sizeErr := app.DirSize(snap.WorkDir)
	newFiles := filesInDir(snap.WorkDir)
	defer cleanupBotWorkDir(snap.WorkDir)
	if sizeErr != nil {
		b.logError(fmt.Sprintf("download dir size %s", logChatUser(chatID, snap.UserID)), sizeErr)
	}

	if sizeErr == nil && actualSize > b.downloadSizeLimit(snap.UserID) {
		b.logf("download limit exceeded after finish %s size=%d limit=%d files=%d", logChatUser(chatID, snap.UserID), actualSize, b.downloadSizeLimit(snap.UserID), len(newFiles))
		b.notifyDownloadSizeLimitExceeded(chatID, msgID, snap.UserID)
		return
	}
	b.logf("download finished %s ok=%d failed=%d total=%d files=%d size=%d", logChatUser(chatID, snap.UserID), summary.OK, summary.Failed, summary.Total, len(newFiles), actualSize)

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
	b.logf("single send summary chat=%d sent=%d too_large=%d send_err=%d", chatID, sent, tooLarge, sendErr)
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
	b.logf("playlist send summary chat=%d sent=%d too_large=%d send_err=%d", chatID, sent, tooLarge, sendErr)
	b.edit(chatID, msgID, b.playlistSendSummary(icon, summary.OK, summary.Failed, summary.Total, sent, tooLarge, sendErr))
}
