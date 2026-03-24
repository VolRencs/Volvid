package bot

import (
	"log"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) startDepsUpdate(chatID int64) {
	b.mu.Lock()
	if b.depsUpdating {
		b.mu.Unlock()
		b.send(chatID, "⏳ Обновление зависимостей уже запущено.")
		return
	}
	if b.sessions.hasBusy() {
		b.mu.Unlock()
		b.send(chatID, "⚠️ Сейчас есть активные загрузки. Останови их и повтори /update.")
		return
	}
	b.depsUpdating = true
	b.mu.Unlock()

	b.send(chatID, "⏳ Обновляю зависимости бота…")

	go func() {
		defer func() {
			b.mu.Lock()
			b.depsUpdating = false
			b.mu.Unlock()
		}()

		if err := app.InstallAllDeps(nil); err != nil {
			log.Printf("bot deps update: %v", err)
			b.send(chatID, "⚠️ Не удалось обновить зависимости:\n<code>"+escapeHTML(err.Error())+"</code>")
			return
		}

		deps := app.DetectDeps()

		b.mu.Lock()
		b.deps = deps
		b.mu.Unlock()

		text := "✅ Зависимости обновлены.\n\n" +
			verLine("yt-dlp", deps.YtdlpVer) + "\n" +
			verLine("ffmpeg", deps.FFmpegVer)
		b.send(chatID, text)
	}()
}

func (b *Bot) helpText(msg *models.Message) string {
	lines := []string{
		"👋 <b>VolRen Downloader Bot</b> v" + app.Version,
		"",
		"Пришли ссылку на YouTube — скачаю видео или аудио.",
		"",
		"<b>Команды:</b>",
		"/cancel — отменить скачивание",
		"/help — справка",
	}

	if b.isAdminMessage(msg) {
		lines = append(lines, "/status — версии yt-dlp и ffmpeg")
	}
	if b.isOwnerMessage(msg) {
		lines = append(lines, "/update — обновить зависимости")
	}

	return stringsJoinLines(lines)
}

func stringsJoinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n" + line
	}
	return out
}
