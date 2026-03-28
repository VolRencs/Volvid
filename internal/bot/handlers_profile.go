package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) handleModeCallback(chatID int64, msgID int, sess *Session, data string) string {
	switch data {
	case cbModeVideo:
		b.setSessionDownloadChoice(sess, StateAwaitingMode, app.ModeVideo, app.OutputProfile{}, msgID)
		b.scanAndAskQuality(chatID, sess)
	case cbModeAudio:
		b.setSessionDownloadChoice(sess, StateAwaitingAudioProfile, app.ModeAudio, app.OutputProfile{}, msgID)
		b.askAudioProfiles(chatID, sess)
	case cbModeThumb:
		sess.mutate(func(s *Session) {
			s.Mode = app.ModeThumbnail
			s.Profile = app.ThumbnailOutputProfile(app.LocaleRU)
			s.StatusMsgID = msgID
		})
		return b.startConfiguredDownloadAlert(chatID, sess)
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
	return b.startConfiguredDownloadAlert(chatID, sess)
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
	return b.startConfiguredDownloadAlert(chatID, sess)
}
