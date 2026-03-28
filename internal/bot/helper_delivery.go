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
