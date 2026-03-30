package bot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	app "YouTubeBuild/internal/app"
)

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

type deliveryStats struct {
	Sent     int
	TooLarge int
	Failed   int
}

func (s *deliveryStats) add(err error) {
	switch {
	case err == nil:
		s.Sent++
	case errors.Is(err, errTelegramFileTooLarge):
		s.TooLarge++
	default:
		s.Failed++
	}
}

func (s deliveryStats) hasIssues() bool {
	return s.TooLarge > 0 || s.Failed > 0
}

func (s deliveryStats) total() int {
	return s.Sent + s.TooLarge + s.Failed
}

func (s deliveryStats) cleanupText() string {
	if s.total() == 1 {
		return "Файл удалён с сервера."
	}
	return "Файлы удалены с сервера."
}

func (b *Bot) sendDownloadedFiles(chatID int64, paths []string) deliveryStats {
	var stats deliveryStats
	for _, path := range paths {
		stats.add(b.sendFile(chatID, path))
	}
	return stats
}

func (b *Bot) singleSendSummary(stats deliveryStats) string {
	switch {
	case stats.Sent > 0 && !stats.hasIssues():
		return "Готово. Файл удалён с сервера."
	case stats.Sent > 0 || stats.TooLarge > 0:
		return fmt.Sprintf(
			"Скачивание завершено.\n\n%s\n\n%s",
			b.deliveryBreakdownText(stats), stats.cleanupText(),
		)
	default:
		return "Скачивание завершено.\n\nНе удалось отправить файл в Telegram.\nФайл удалён с сервера."
	}
}

func (b *Bot) playlistSendSummary(icon string, summary doneSummary, stats deliveryStats) string {
	var extra string
	if stats.Sent > 0 || stats.TooLarge > 0 || stats.Failed > 0 {
		extra = "\n\n" + b.deliveryBreakdownText(stats) + "\nВсе файлы удалены с сервера."
	}

	return fmt.Sprintf(
		"%s<b>Плейлист завершён</b>\n\nУспешно: %d\nОшибок: %d\nИтого: %d%s",
		icon, summary.OK, summary.Failed, summary.Total, extra,
	)
}

func (b *Bot) deliveryBreakdownText(stats deliveryStats) string {
	lines := []string{
		fmt.Sprintf("Отправлено: %d", stats.Sent),
		fmt.Sprintf("Слишком большие для %s: %d", b.backendLabel(), stats.TooLarge),
		fmt.Sprintf("Не удалось отправить: %d", stats.Failed),
	}
	if stats.TooLarge > 0 {
		lines = append(lines, "", b.sendLimitNotice())
	}
	return strings.Join(lines, "\n")
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
