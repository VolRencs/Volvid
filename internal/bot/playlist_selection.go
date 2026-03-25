package bot

import (
	"fmt"
	"strconv"
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

const playlistSelectionPageSize = 5

func (b *Bot) kbPlaylistSelection(sess *Session) models.InlineKeyboardMarkup {
	snap := sess.snapshot()
	rows := make([][]models.InlineKeyboardButton, 0, playlistSelectionPageSize+4)
	if snap.PlInfo == nil || len(snap.PlInfo.Entries) == 0 {
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton("❌ Отмена", cbCancel),
		})
		return models.InlineKeyboardMarkup{InlineKeyboard: rows}
	}

	page, start, end, pageCount := playlistSelectionPageBounds(snap.PlInfo, snap.PlaylistPage)
	for _, entry := range snap.PlInfo.Entries[start:end] {
		mark := "⬜️"
		if snap.SelectedIndices[entry.Index] {
			mark = "✅"
		}
		label := fmt.Sprintf("%s %d. %s", mark, entry.Index, truncateButtonLabel(entry.Title, 30))
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton(label, cbPlTogglePref+strconv.Itoa(entry.Index)),
		})
	}

	if pageCount > 1 {
		prevData := cbNoop
		nextData := cbNoop
		if page > 0 {
			prevData = cbPlPagePref + strconv.Itoa(page-1)
		}
		if page+1 < pageCount {
			nextData = cbPlPagePref + strconv.Itoa(page+1)
		}
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton("◀️", prevData),
			kbButton(fmt.Sprintf("%d/%d", page+1, pageCount), cbNoop),
			kbButton("▶️", nextData),
		})
	}

	rows = append(rows, []models.InlineKeyboardButton{
		kbButton("☑️ Все", cbPlSelectAll),
		kbButton("⬜️ Снять", cbPlSelectNone),
	})

	doneLabel := "✅ Готово"
	if selected := len(snap.SelectedIndices); selected > 0 {
		doneLabel = fmt.Sprintf("✅ Готово (%d/%d)", selected, b.playlistItemLimit(snap.UserID))
	}
	rows = append(rows, []models.InlineKeyboardButton{
		kbButton(doneLabel, cbPlSelectDone),
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

	total := len(snap.PlInfo.Entries)
	limit := b.playlistItemLimit(snap.UserID)
	page, start, end, pageCount := playlistSelectionPageBounds(snap.PlInfo, snap.PlaylistPage)

	var scope string
	if total > 0 {
		scope = fmt.Sprintf(
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

	return fmt.Sprintf(
		"🎯 <b>%s</b>\nВыбрано: %d / %d%s\n\nНажми на нужные видео ниже, затем нажми «Готово».",
		escapeHTML(snap.PlInfo.Title),
		len(snap.SelectedIndices),
		limit,
		scope,
	)
}

func (b *Bot) openPlaylistSelection(chatID int64, msgID int, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateAwaitingPlaylistSelection
		s.StatusMsgID = msgID
		s.SelectedIndices = nil
		s.PlaylistPage = 0
		s.QualityChoices = nil
		s.Profile = app.OutputProfile{}
	})
	b.editKb(chatID, msgID, b.playlistSelectionText(sess), b.kbPlaylistSelection(sess))
}

func (b *Bot) applyPlaylistSelection(sess *Session, selected map[int]bool) {
	sess.mutate(func(s *Session) {
		s.SelectedIndices = cloneSelection(selected)
	})
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
