package bot

import (
	"fmt"
	"strconv"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

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
