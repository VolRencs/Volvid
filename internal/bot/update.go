package bot

import (
	"log"
	"strings"

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

	b.logf("deps update start chat=%d", chatID)
	b.send(chatID, "⏳ Обновляю зависимости бота…")

	go func() {
		defer func() {
			b.mu.Lock()
			b.depsUpdating = false
			b.mu.Unlock()
		}()

		if err := app.InstallAllDepsFor(app.LocaleRU, nil); err != nil {
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
		b.logf("deps update done chat=%d yt-dlp=%q ffmpeg=%q", chatID, deps.YtdlpVer, deps.FFmpegVer)
		b.send(chatID, text)
	}()
}

func (b *Bot) helpText(msg *models.Message) string {
	lines := []string{
		"👋 <b>VolRen Downloader Bot</b> v" + app.Version,
		"",
		"Пришли ссылку на YouTube или просто название видео — скачаю видео, аудио или превью.",
		"Есть выбор качества видео и 5 аудио-пресетов.",
		"По тексту бот выполнит поиск и предложит 3-5 результатов.",
		"Из плейлиста можно выбрать до 5 видео, с премиумом — до 30.",
		"",
		"<b>Команды:</b>",
		"/cancel — отменить скачивание",
		"/help — справка",
		"/premium — купить премиум (лимит 2GB и до 30 видео)",
	}

	if b.isAdminMessage(msg) {
		lines = append(lines,
			"/status — версии yt-dlp и ffmpeg",
			"/broadcast — рассылка текста или reply-копии сообщения",
			"/schedule — таймер рассылки",
			"/timers — активные таймеры",
			"/deltimer — удалить таймер",
		)
	}
	if b.isOwnerMessage(msg) {
		lines = append(lines, "/update — обновить зависимости")
	}

	return strings.Join(lines, "\n")
}
