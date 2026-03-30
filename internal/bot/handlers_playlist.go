package bot

import (
	"fmt"
	"strconv"
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

const (
	activeDownloadText        = "⏳ Уже идёт скачивание. Дождись завершения или отмени /cancel."
	playlistSelectionPageSize = 5
)

func (b *Bot) handlePlaylistOpCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbPlVideo:
		sess.chooseSingleVideo(msgID)
		b.removeKb(chatID, msgID)
		b.probeAndAskFragment(chatID, sess)
	case cbPlChoose:
		sess.setStatusMessage(msgID)
		b.removeKb(chatID, msgID)
		b.fetchAndAskPlaylist(chatID, sess)
	}
	return ""
}

func (b *Bot) handlePlaylistSelectionInput(chatID int64, sess *Session, raw string) {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		b.resetStaleSession(chatID, 0)
		return
	}

	indices, err := app.ParseSelectionFor(raw, len(snap.PlInfo.Entries), app.LocaleRU)
	if err != nil {
		b.send(chatID, "⚠️ "+escapeHTML(err.Error())+"\n\nПример: <code>1-3,7,10</code> или <code>all</code>")
		return
	}

	if !b.selectPlaylistIndices(sess, indices) {
		b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
		return
	}
	b.askMode(chatID, sess)
}

func (b *Bot) handlePlaylistSelectionCallback(chatID int64, msgID int, sess *Session, data string) string {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return staleSessionAlert
	}

	switch {
	case data == cbPlSelectAll:
		if !b.selectAllPlaylistEntries(sess) {
			b.notifyPlaylistItemLimitExceeded(chatID, snap.UserID)
			return b.playlistItemLimitAlert(snap.UserID)
		}
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
		sess.setStatusMessage(msgID)
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
		sess.setPlaylistPage(page)
	default:
		return ""
	}

	b.editKb(chatID, msgID, b.playlistSelectionText(sess), b.kbPlaylistSelection(sess))
	return ""
}

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

func playlistSelectionPageBounds(info *app.PlaylistInfo, page int) (normalized, start, end, pageCount int) {
	if info == nil || len(info.Entries) == 0 {
		return 0, 0, 0, 1
	}

	pageCount = (len(info.Entries) + playlistSelectionPageSize - 1) / playlistSelectionPageSize
	normalized = max(0, min(page, pageCount-1))
	start = normalized * playlistSelectionPageSize
	end = min(len(info.Entries), start+playlistSelectionPageSize)
	return normalized, start, end, pageCount
}

func truncateButtonLabel(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
}

func (b *Bot) applyPlaylistSelection(sess *Session, selected map[int]bool) {
	sess.setPlaylistSelection(selected)
}

func (b *Bot) setPlaylistSelection(sess *Session, selected map[int]bool) bool {
	snap := sess.snapshot()
	if !b.validatePlaylistSelectionCount(snap.UserID, len(selected)) {
		return false
	}
	b.applyPlaylistSelection(sess, selected)
	return true
}

func (b *Bot) selectPlaylistIndices(sess *Session, indices []int) bool {
	return b.setPlaylistSelection(sess, selectionMapFromIndices(indices))
}

func (b *Bot) selectAllPlaylistEntries(sess *Session) bool {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return false
	}
	return b.setPlaylistSelection(sess, selectionMapForAll(snap.PlInfo.Entries))
}

func (b *Bot) togglePlaylistSelection(sess *Session, idx int) (map[int]bool, bool, bool) {
	snap := sess.snapshot()
	selected := cloneSelection(snap.SelectedIndices)
	if selected == nil {
		selected = make(map[int]bool)
	}
	if selected[idx] {
		delete(selected, idx)
		return selected, true, false
	}
	if idx < 1 || idx > len(snap.PlInfo.Entries) {
		return selected, false, false
	}
	if !b.validatePlaylistSelectionCount(snap.UserID, len(selected)+1) {
		return selected, false, true
	}
	selected[idx] = true
	return selected, true, false
}

func (b *Bot) playlistSelectionEntries(sess *Session) []app.PlaylistEntry {
	snap := sess.snapshot()
	if snap.PlInfo == nil || snap.ForceSingle || len(snap.SelectedIndices) == 0 {
		return nil
	}
	return playlistEntriesFromSelection(snap.PlInfo, snap.SelectedIndices)
}

func (b *Bot) selectedPlaylistCount(sess *Session) int {
	return len(sess.snapshot().SelectedIndices)
}

func (b *Bot) playlistSelectionURLs(sess *Session) []string {
	entries := b.playlistSelectionEntries(sess)
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}
	return urls
}

func selectionMapFromIndices(indices []int) map[int]bool {
	if len(indices) == 0 {
		return nil
	}
	selected := make(map[int]bool, len(indices))
	for _, idx := range indices {
		selected[idx] = true
	}
	return selected
}

func selectionMapForAll(entries []app.PlaylistEntry) map[int]bool {
	if len(entries) == 0 {
		return nil
	}
	selected := make(map[int]bool, len(entries))
	for _, entry := range entries {
		selected[entry.Index] = true
	}
	return selected
}

func playlistEntriesFromSelection(info *app.PlaylistInfo, selected map[int]bool) []app.PlaylistEntry {
	if info == nil || len(selected) == 0 {
		return nil
	}

	entries := make([]app.PlaylistEntry, 0, len(selected))
	for _, entry := range info.Entries {
		if selected[entry.Index] {
			entries = append(entries, entry)
		}
	}
	return entries
}

func playlistSelectionCount(snap SessionSnapshot) int {
	return len(snap.SelectedIndices)
}

func (b *Bot) kbPlaylistSelection(sess *Session) models.InlineKeyboardMarkup {
	snap := sess.snapshot()
	rows := make([][]models.InlineKeyboardButton, 0, playlistSelectionPageSize+4)
	if snap.PlInfo == nil || len(snap.PlInfo.Entries) == 0 {
		rows = append(rows, []models.InlineKeyboardButton{kbButton("❌ Отмена", cbCancel)})
		return models.InlineKeyboardMarkup{InlineKeyboard: rows}
	}

	page, start, end, pageCount := playlistSelectionPageBounds(snap.PlInfo, snap.PlaylistPage)
	for _, entry := range snap.PlInfo.Entries[start:end] {
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton(b.playlistSelectionEntryLabel(snap, entry), cbPlTogglePref+strconv.Itoa(entry.Index)),
		})
	}

	if pageCount > 1 {
		rows = append(rows, b.playlistSelectionPaginationRow(page, pageCount))
	}
	rows = append(rows, []models.InlineKeyboardButton{
		kbButton("☑️ Все", cbPlSelectAll),
		kbButton("⬜️ Снять", cbPlSelectNone),
	})
	rows = append(rows, []models.InlineKeyboardButton{
		kbButton(b.playlistSelectionDoneLabel(snap), cbPlSelectDone),
	})
	rows = append(rows, []models.InlineKeyboardButton{
		kbButton("❌ Отмена", cbCancel),
	})
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) playlistSelectionText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return "Пришли ссылку заново."
	}

	return fmt.Sprintf(
		"🎯 <b>%s</b>\nВыбрано: %d / %d%s\n\nНажми на нужные видео ниже, затем нажми «Готово».",
		escapeHTML(snap.PlInfo.Title),
		len(snap.SelectedIndices),
		b.playlistItemLimit(snap.UserID),
		b.playlistSelectionScopeText(snap),
	)
}

func (b *Bot) openPlaylistSelection(chatID int64, msgID int, sess *Session) {
	sess.beginPlaylistSelection(msgID)
	b.editKb(chatID, msgID, b.playlistSelectionText(sess), b.kbPlaylistSelection(sess))
}

func (b *Bot) playlistSelectionEntryLabel(snap SessionSnapshot, entry app.PlaylistEntry) string {
	mark := "⬜️"
	if snap.SelectedIndices[entry.Index] {
		mark = "✅"
	}
	return fmt.Sprintf("%s %d. %s", mark, entry.Index, truncateButtonLabel(entry.Title, 30))
}

func (b *Bot) playlistSelectionPaginationRow(page, pageCount int) []models.InlineKeyboardButton {
	prevData := cbNoop
	nextData := cbNoop
	if page > 0 {
		prevData = cbPlPagePref + strconv.Itoa(page-1)
	}
	if page+1 < pageCount {
		nextData = cbPlPagePref + strconv.Itoa(page+1)
	}
	return []models.InlineKeyboardButton{
		kbButton("◀️", prevData),
		kbButton(fmt.Sprintf("%d/%d", page+1, pageCount), cbNoop),
		kbButton("▶️", nextData),
	}
}

func (b *Bot) playlistSelectionDoneLabel(snap SessionSnapshot) string {
	if selected := len(snap.SelectedIndices); selected > 0 {
		return fmt.Sprintf("✅ Готово (%d/%d)", selected, b.playlistItemLimit(snap.UserID))
	}
	return "✅ Готово"
}

func (b *Bot) playlistSelectionScopeText(snap SessionSnapshot) string {
	if snap.PlInfo == nil || len(snap.PlInfo.Entries) == 0 {
		return ""
	}

	total := len(snap.PlInfo.Entries)
	page, start, end, pageCount := playlistSelectionPageBounds(snap.PlInfo, snap.PlaylistPage)
	return fmt.Sprintf(
		"\nВсего в плейлисте: %d\nЛимит аккаунта: %s\nСтраница: %d/%d · %d-%d из %d",
		total,
		b.playlistItemLimitText(snap.UserID),
		page+1,
		pageCount,
		start+1,
		end,
		total,
	)
}
