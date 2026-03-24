package bot

import (
	"fmt"
	"strconv"
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

const playlistSelectionPageSize = 5

func kbPlaylistSelection(sess *Session) models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, playlistSelectionPageSize+4)
	if sess == nil || sess.PlInfo == nil || len(sess.PlInfo.Entries) == 0 {
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton("❌ Отмена", cbCancel),
		})
		return models.InlineKeyboardMarkup{InlineKeyboard: rows}
	}

	page, start, end, pageCount := playlistSelectionPageBounds(sess.PlInfo, sess.PlaylistPage)
	for _, entry := range sess.PlInfo.Entries[start:end] {
		mark := "⬜️"
		if sess.SelectedIndices[entry.Index] {
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
	if selected := len(sess.SelectedIndices); selected > 0 {
		doneLabel = fmt.Sprintf("✅ Готово (%d)", selected)
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
	if sess == nil || sess.PlInfo == nil {
		return "Пришли ссылку заново."
	}

	total := len(sess.PlInfo.Entries)
	page, start, end, pageCount := playlistSelectionPageBounds(sess.PlInfo, sess.PlaylistPage)
	sess.PlaylistPage = page

	var scope string
	if total > 0 {
		scope = fmt.Sprintf("\nСтраница: %d/%d · %d-%d из %d", page+1, pageCount, start+1, end, total)
	}

	return fmt.Sprintf(
		"🎯 <b>%s</b>\nВыбрано: %d / %d%s\n\nНажми на нужные видео ниже, затем нажми «Готово».",
		escapeHTML(sess.PlInfo.Title),
		len(sess.SelectedIndices),
		total,
		scope,
	)
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
