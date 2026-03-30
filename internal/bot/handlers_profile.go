package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) handleModeCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbModeVideo:
		sess.chooseMode(app.ModeVideo, msgID)
		b.scanAndAskQuality(chatID, sess)
	case cbModeAudio:
		sess.chooseMode(app.ModeAudio, msgID)
		b.askAudioProfiles(chatID, sess)
	case cbModeThumb:
		sess.chooseMode(app.ModeThumbnail, msgID)
		sess.setProfile(app.ThumbnailOutputProfile(app.LocaleRU), msgID)
		return b.startConfiguredDownloadText(chatID, sess)
	}
	return ""
}

func (b *Bot) handleAudioProfileCallback(chatID int64, msgID int, sess *Session, data string) string {
	if !strings.HasPrefix(data, cbAudioPrefix) {
		return ""
	}

	profiles := app.AudioOutputProfiles(app.LocaleRU)
	profile, ok := app.FindOutputProfile(profiles, strings.TrimPrefix(data, cbAudioPrefix))
	if !ok {
		return ""
	}

	sess.setProfile(profile, msgID)
	return b.startConfiguredDownloadText(chatID, sess)
}

func (b *Bot) handleQualityCallback(chatID int64, msgID int, sess *Session, data string) string {
	if !strings.HasPrefix(data, cbQualityPrefix) {
		return ""
	}
	choices := sess.snapshot().QualityChoices
	if len(choices) == 0 {
		choices = app.DefaultQualityChoices()
	}
	choice, ok := app.FindQualityChoice(choices, strings.TrimPrefix(data, cbQualityPrefix))
	if !ok {
		return ""
	}

	sess.setProfile(choice.Profile(app.LocaleRU), msgID)
	return b.startConfiguredDownloadText(chatID, sess)
}

func (b *Bot) startConfiguredDownloadText(chatID int64, sess *Session) string {
	if err := b.startConfiguredDownload(chatID, sess); err != nil {
		if err == errDownloadLimitExceeded {
			return ""
		}
		return err.Error()
	}
	return ""
}
