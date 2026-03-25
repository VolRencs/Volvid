package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) handleSearch(chatID, userID int64, query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		b.send(chatID, "⚠️ Введи название видео для поиска.")
		return
	}

	sess := b.sessions.get(chatID)
	if sess != nil && sess.snapshot().State == StateDownloading {
		b.send(chatID, "⏳ Уже идёт скачивание. Дождись завершения или отмени /cancel.")
		return
	}

	searchSess := newSession(userID, "", "")
	searchSess.mutate(func(s *Session) {
		s.SearchQuery = query
	})
	b.sessions.set(chatID, searchSess)

	statusMsg, _ := b.send(chatID, "🔎 Ищу видео на YouTube…")
	searchSess.mutate(func(s *Session) {
		s.StatusMsgID = statusMsg.ID
	})

	go func() {
		results, err := app.SearchYouTubeContext(context.Background(), query)
		if err != nil || len(results) == 0 {
			b.edit(chatID, statusMsg.ID, "⚠️ Не удалось найти видео по запросу.\n\nПопробуй уточнить название.")
			b.sessions.reset(chatID)
			return
		}

		searchSess.mutate(func(s *Session) {
			s.State = StateAwaitingSearchSelection
			s.SearchResults = results
			s.SearchQuery = query
		})
		b.editKb(chatID, statusMsg.ID, b.searchResultsText(searchSess), kbSearchResults(searchSess))
	}()
}

func (b *Bot) handleSearchCallback(chatID int64, sess *Session, data string, userID int64) string {
	if !strings.HasPrefix(data, cbSearchPrefix) {
		return ""
	}

	idx, err := strconv.Atoi(strings.TrimPrefix(data, cbSearchPrefix))
	if err != nil {
		return "Не удалось прочитать выбранный результат."
	}

	results := sess.snapshot().SearchResults
	if idx < 0 || idx >= len(results) {
		return "Результат поиска устарел. Выполни поиск заново."
	}

	go b.handleURL(chatID, userID, results[idx].URL)
	return ""
}

func kbSearchResults(sess *Session) models.InlineKeyboardMarkup {
	snap := sess.snapshot()
	rows := make([][]models.InlineKeyboardButton, 0, len(snap.SearchResults)+1)
	for idx, result := range snap.SearchResults {
		label := fmt.Sprintf("%d. %s", idx+1, truncateButtonLabel(result.Title, 28))
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton(label, cbSearchPrefix+strconv.Itoa(idx)),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{{Text: "❌ Отмена", CallbackData: cbCancel}})
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) searchResultsText(sess *Session) string {
	snap := sess.snapshot()
	lines := []string{
		fmt.Sprintf("🔎 <b>Результаты для:</b> %s", escapeHTML(snap.SearchQuery)),
		"",
	}

	for idx, result := range snap.SearchResults {
		title := escapeHTML(result.Title)
		line := fmt.Sprintf("%d. <a href=\"%s\">%s</a>", idx+1, escapeHTML(result.URL), title)
		if result.Duration > 0 {
			line += "  " + escapeHTML(app.FmtDuration(result.Duration))
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", "Выбери видео кнопкой ниже.")
	return strings.Join(lines, "\n")
}
