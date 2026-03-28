package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) handleModeCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbModeVideo:
		sess.mutate(func(s *Session) {
			s.State = StateAwaitingMode
			s.Mode = app.ModeVideo
			s.Profile = app.OutputProfile{}
			s.StatusMsgID = msgID
		})
		b.scanAndAskQuality(chatID, sess)
	case cbModeAudio:
		sess.mutate(func(s *Session) {
			s.State = StateAwaitingAudioProfile
			s.Mode = app.ModeAudio
			s.Profile = app.OutputProfile{}
			s.StatusMsgID = msgID
		})
		b.askAudioProfiles(chatID, sess)
	case cbModeThumb:
		sess.mutate(func(s *Session) {
			s.Mode = app.ModeThumbnail
			s.Profile = app.ThumbnailOutputProfile(app.LocaleRU)
			s.StatusMsgID = msgID
		})
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

	sess.mutate(func(s *Session) {
		s.Profile = profile
		s.StatusMsgID = msgID
	})
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

	sess.mutate(func(s *Session) {
		s.Profile = choice.Profile(app.LocaleRU)
		s.StatusMsgID = msgID
	})
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
