package bot

import (
	"fmt"

	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) scanAndAskQuality(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateFetchingQuality
	})

	b.upsertSessionText(chatID, sess, b.qualityScanText(sess))
	urls := b.qualityScanURLs(sess)
	go b.runQualityScan(chatID, sess, urls)
}

func (b *Bot) runQualityScan(chatID int64, sess *Session, urls []string) {
	choices := b.resolveQualityChoices(chatID, sess, urls)
	if sess.isCancelled() {
		snap := sess.snapshot()
		b.logf("quality scan cancelled %s", logChatUser(chatID, snap.UserID))
		return
	}

	snap := sess.snapshot()
	b.logf("quality choices ready %s count=%d", logChatUser(chatID, snap.UserID), len(choices))
	sess.mutate(func(s *Session) {
		s.QualityChoices = choices
		s.State = StateAwaitingQuality
	})
	b.upsertSessionKeyboard(chatID, sess, b.qualityPromptText(sess), kbQuality(choices))
}

func (b *Bot) resolveQualityChoices(chatID int64, sess *Session, urls []string) []app.QualityChoice {
	var choices []app.QualityChoice
	var err error
	if len(urls) > 0 {
		choices, err = app.ResolveQualityChoices(urls)
	}
	if err != nil {
		snap := sess.snapshot()
		b.logError(fmt.Sprintf("quality scan %s", logChatUser(chatID, snap.UserID)), err)
	}
	if len(choices) == 0 {
		return app.DefaultQualityChoices()
	}
	return choices
}

func (b *Bot) askMode(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateAwaitingMode
		s.Mode = app.DefaultDownloadMode()
		s.Profile = app.DefaultVideoProfile(app.LocaleRU)
		s.QualityChoices = nil
		if s.PlInfo != nil && !s.ForceSingle {
			s.Fragment = nil
		}
	})

	b.upsertSessionKeyboard(chatID, sess, b.modePromptText(sess), kbMode())
}

func (b *Bot) askFragmentChoice(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateAwaitingFragmentChoice
		s.Fragment = nil
	})

	snap := sess.snapshot()
	b.upsertSessionKeyboard(chatID, sess, b.fragmentPromptText(sess), kbFragmentChoice(snap.Target.HasURLStart && snap.Target.URLStartAt > 0))
}

func (b *Bot) askAudioProfiles(chatID int64, sess *Session) {
	profiles := app.AudioOutputProfiles(app.LocaleRU)
	sess.mutate(func(s *Session) {
		s.State = StateAwaitingAudioProfile
		s.Profile = app.OutputProfile{}
	})
	b.upsertSessionKeyboard(chatID, sess, b.audioPromptText(sess), kbAudioProfiles(profiles))
}

func (b *Bot) upsertSessionText(chatID int64, sess *Session, text string) {
	snap := sess.snapshot()
	if snap.StatusMsgID == 0 {
		msg, _ := b.send(chatID, text)
		if msg.ID != 0 {
			sess.mutate(func(s *Session) {
				s.StatusMsgID = msg.ID
			})
		}
		return
	}
	b.edit(chatID, snap.StatusMsgID, text)
}

func (b *Bot) upsertSessionKeyboard(chatID int64, sess *Session, text string, kb models.InlineKeyboardMarkup) {
	snap := sess.snapshot()
	if snap.StatusMsgID == 0 {
		msg, _ := b.sendKb(chatID, text, kb)
		if msg.ID != 0 {
			sess.mutate(func(s *Session) {
				s.StatusMsgID = msg.ID
			})
		}
		return
	}
	b.editKb(chatID, snap.StatusMsgID, text, kb)
}
