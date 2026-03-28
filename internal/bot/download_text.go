package bot

import (
	"fmt"
	"strings"

	app "YouTubeBuild/internal/app"
)

func playlistResultIcon(done, failed int) string {
	switch {
	case failed > 0 && done == 0:
		return "❌"
	case failed > 0:
		return "⚠️"
	default:
		return "✅"
	}
}

func formatFragmentLabel(fragment *app.DownloadFragment) string {
	return app.FormatFragmentLabel(fragment)
}

func (b *Bot) fragmentPromptText(sess *Session) string {
	snap := sess.snapshot()
	lines := []string{"✂️ <b>Фрагмент</b>", "", "Выбери вариант загрузки:"}
	if durationText := app.FragmentDurationText(app.LocaleRU, snap.MediaDuration); durationText != "" {
		lines = append(lines, "", escapeHTML(durationText))
	}
	if b.allowURLStartFragment(snap) {
		lines = append(lines, "", fmt.Sprintf("URL таймкод: <code>%s</code>", app.FormatClockTimestamp(snap.Target.URLStartAt)))
	}
	return strings.Join(lines, "\n")
}

func (b *Bot) fragmentInputHint(mediaDuration int) string {
	return escapeHTML(app.FragmentInputHintFor(app.LocaleRU, mediaDuration))
}

func (b *Bot) fragmentInputText(mediaDuration int) string {
	return escapeHTML(strings.TrimSpace(app.StringsFor(app.LocaleRU).FragmentInputPrompt)) + "\n\n" + b.fragmentInputHint(mediaDuration)
}

func (b *Bot) fragmentModeNoticeText(notice string, sess *Session) string {
	if strings.TrimSpace(notice) == "" {
		return b.modePromptText(sess)
	}
	return "⚠️ " + escapeHTML(notice) + "\n\n" + b.modePromptText(sess)
}

func (b *Bot) allowURLStartFragment(snap SessionSnapshot) bool {
	return snap.Target.HasURLStart && snap.Target.URLStartAt > 0 && snap.MediaDuration > 0 && snap.Target.URLStartAt < snap.MediaDuration
}

func (b *Bot) downloadProgressText(done, failed, total int, dlStem string, update app.DlUpdate) string {
	if total > 0 {
		completed := done + failed
		return fmt.Sprintf(
			"⬇️ Плейлист: %s\n%d / %d  (%.1f%%)\n%s  %s",
			progressBar(completed, total), completed, total,
			float64(completed)/float64(total)*100,
			app.FmtBytesFor(update.DoneB, app.LocaleRU), update.Speed,
		)
	}
	return fmt.Sprintf(
		"⬇️ <b>%s</b>\n%.1f%%  %s  %s",
		escapeHTML(dlStem), update.Pct,
		app.FmtBytesFor(update.DoneB, app.LocaleRU), update.Speed,
	)
}

func (b *Bot) qualityScanText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		count := playlistSelectionCount(snap)
		if count == 0 {
			return "🔍 Сканирую качества и размер…"
		}
		if !app.ShouldScanQualityChoices(count) {
			return fmt.Sprintf("⚡ Готовлю быстрый выбор качества для %d видео…", count)
		}
		return fmt.Sprintf("🔍 Сканирую качества и размер для %d видео…", count)
	}
	return "🔍 Сканирую качества и размер…"
}

func (b *Bot) qualityPromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		count := playlistSelectionCount(snap)
		return fmt.Sprintf(
			"📋 <b>%s</b>\n%d видео\n\nВыбери качество видео:",
			escapeHTML(snap.PlInfo.Title),
			count,
		)
	}
	return "🎬 Выбери качество видео:"
}

func (b *Bot) qualityScanURLs(sess *Session) []string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle && playlistSelectionCount(snap) == 0 {
		return nil
	}
	if snap.PlInfo == nil || snap.ForceSingle {
		return []string{snap.Target.DownloadURL(snap.ForceSingle)}
	}
	return b.playlistSelectionURLs(sess)
}

func (b *Bot) modePromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf("📋 <b>%s</b>\n%d видео\n\nЧто скачать?", escapeHTML(snap.PlInfo.Title), playlistSelectionCount(snap))
	}
	return "🎛 <b>Выбери режим</b>\n\nЧто скачать?"
}

func (b *Bot) audioPromptText(sess *Session) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return fmt.Sprintf("📋 <b>%s</b>\n%d видео\n\nВыбери аудио:", escapeHTML(snap.PlInfo.Title), playlistSelectionCount(snap))
	}
	return "🎵 Выбери аудио:"
}

func (b *Bot) downloadStartText(sess *Session) string {
	snap := sess.snapshot()
	profile := snap.Profile
	if profile.Mode == 0 {
		profile = app.DefaultProfileForMode(snap.Mode, app.LocaleRU)
	}

	modeIcon, modeLabel := downloadModeDisplay(profile)

	if snap.PlInfo != nil && !snap.ForceSingle {
		count := playlistSelectionCount(snap)
		return fmt.Sprintf(
			"📋 <b>%s</b>\n%d видео\n🎛 Режим: %s\n\n⬇️ Начинаю скачивание…",
			escapeHTML(snap.PlInfo.Title),
			count,
			escapeHTML(modeLabel),
		)
	}
	fragment := formatFragmentLabel(snap.Fragment)
	if fragment != "" && profile.Mode != app.ModeThumbnail {
		return fmt.Sprintf("%s %s\n✂️ Фрагмент: <code>%s</code>\n\n⬇️ Начинаю скачивание…", modeIcon, escapeHTML(modeLabel), escapeHTML(fragment))
	}
	return fmt.Sprintf("%s %s\n\n⬇️ Начинаю скачивание…", modeIcon, escapeHTML(modeLabel))
}

func downloadModeDisplay(profile app.OutputProfile) (string, string) {
	if profile.Label != "" {
		switch profile.Mode {
		case app.ModeAudio:
			return "🎵", profile.Label
		case app.ModeThumbnail:
			return "🖼", profile.Label
		default:
			return "🎬", profile.Label
		}
	}

	switch profile.Mode {
	case app.ModeAudio:
		return "🎵", "Аудио"
	case app.ModeThumbnail:
		return "🖼", "Превью"
	default:
		return "🎬", "Видео"
	}
}
