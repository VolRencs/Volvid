package bot

import (
	"context"
	"fmt"
	"time"

	app "YouTubeBuild/internal/app"
)

const downloadEditThrottle = 2 * time.Second

type doneSummary struct {
	Stem    string
	OK      int
	Failed  int
	Total   int
	ErrText string
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

	state := doneSummary{Total: len(req.Entries)}
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
		b.edit(chatID, msgID, "Скачивание отменено.")
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
	case app.EvStart:
		state.Stem = update.Text
	case app.EvDest:
		state.Stem = update.Text
	case app.EvProgress:
		if editor.Allow(time.Now()) {
			b.edit(chatID, msgID, b.downloadProgressText(state.OK, state.Failed, state.Total, state.Stem, update))
		}
	case app.EvProc:
		if editor.Allow(time.Now()) {
			b.edit(chatID, msgID, escapeHTML(update.Text))
		}
	case app.EvDone:
		if update.OK {
			state.OK++
		} else {
			state.Failed++
			if state.ErrText == "" {
				state.ErrText = update.ErrText
			}
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
	newFiles := filesInDir(snap.WorkDir)
	actualSize, sizeErr := app.DirSize(snap.WorkDir)
	if sizeErr != nil {
		b.logError(fmt.Sprintf("download dir size %s", logChatUser(chatID, snap.UserID)), sizeErr)
	}

	limit := b.downloadSizeLimit(snap.UserID)
	if sizeErr == nil && actualSize > limit {
		b.logf("download limit exceeded after finish %s size=%d limit=%d files=%d", logChatUser(chatID, snap.UserID), actualSize, limit, len(newFiles))
		cleanupBotWorkDir(snap.WorkDir)
		b.notifyDownloadSizeLimitExceeded(chatID, msgID, snap.UserID)
		return
	}
	b.logf("download finished %s ok=%d failed=%d total=%d files=%d size=%d", logChatUser(chatID, snap.UserID), summary.OK, summary.Failed, summary.Total, len(newFiles), actualSize)

	text := b.downloadCompletionText(chatID, msgID, newFiles, summary)
	cleanupBotWorkDir(snap.WorkDir)
	b.edit(chatID, msgID, text)
}

func (b *Bot) downloadCompletionText(chatID int64, msgID int, newFiles []string, summary doneSummary) string {
	if summary.Total == 0 {
		return b.singleDownloadCompletionText(chatID, msgID, newFiles, summary)
	}
	return b.playlistDownloadCompletionText(chatID, msgID, newFiles, summary)
}

func (b *Bot) singleDownloadCompletionText(chatID int64, msgID int, newFiles []string, summary doneSummary) string {
	failed := summary.Failed > 0 || summary.OK == 0
	if failed {
		if summary.ErrText != "" {
			return "Не удалось скачать видео.\n\n" + escapeHTML(summary.ErrText)
		}
		if len(newFiles) > 0 {
			return "Не удалось скачать видео. Временные файлы удалены."
		}
		return "Не удалось скачать видео."
	}
	if len(newFiles) == 0 {
		return "Скачивание завершено."
	}

	b.edit(chatID, msgID, "Скачивание завершено. Отправляю файл…")
	stats := b.sendDownloadedFiles(chatID, newFiles)
	b.logf("single send summary chat=%d sent=%d too_large=%d send_err=%d", chatID, stats.Sent, stats.TooLarge, stats.Failed)
	return b.singleSendSummary(stats)
}

func (b *Bot) playlistDownloadCompletionText(chatID int64, msgID int, newFiles []string, summary doneSummary) string {
	icon := playlistResultIcon(summary.OK, summary.Failed)
	if len(newFiles) == 0 {
		text := fmt.Sprintf(
			"%s<b>Плейлист завершён</b>\n\nУспешно: %d\nОшибок: %d\nИтого: %d",
			icon, summary.OK, summary.Failed, summary.Total,
		)
		if summary.ErrText != "" {
			text += "\n\n" + escapeHTML(summary.ErrText)
		}
		return text
	}

	b.edit(chatID, msgID, fmt.Sprintf(
		"%s<b>Плейлист завершён</b>\n\nУспешно: %d\nОшибок: %d\nИтого: %d\n\nОтправляю файлы в Telegram…",
		icon, summary.OK, summary.Failed, summary.Total,
	))
	stats := b.sendDownloadedFiles(chatID, newFiles)
	b.logf("playlist send summary chat=%d sent=%d too_large=%d send_err=%d", chatID, stats.Sent, stats.TooLarge, stats.Failed)
	return b.playlistSendSummary(icon, summary, stats)
}
