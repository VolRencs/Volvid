package bot

import (
	"context"
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
	b.askModeWithNotice(chatID, sess, "")
}

func (b *Bot) askModeWithNotice(chatID int64, sess *Session, notice string) {
	sess.mutate(func(s *Session) {
		s.State = StateAwaitingMode
		s.Mode = app.DefaultDownloadMode()
		s.Profile = app.DefaultVideoProfile(app.LocaleRU)
		s.QualityChoices = nil
		if s.PlInfo != nil && !s.ForceSingle {
			s.Fragment = nil
		}
	})

	b.upsertSessionKeyboard(chatID, sess, b.fragmentModeNoticeText(notice, sess), kbMode())
}

func (b *Bot) probeAndAskFragment(chatID int64, sess *Session) {
	sess.mutate(func(s *Session) {
		s.State = StateFetchingFragmentMetadata
		s.MediaDuration = 0
		s.Fragment = nil
	})
	b.upsertSessionText(chatID, sess, "⏳ Определяю длительность видео…")
	go b.runFragmentProbe(chatID, sess)
}

func (b *Bot) runFragmentProbe(chatID int64, sess *Session) {
	snap := sess.snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	defer close(done)

	go func() {
		stopCh := sess.stopSignal()
		if stopCh == nil {
			return
		}
		select {
		case <-stopCh:
			cancel()
		case <-done:
		}
	}()

	duration, err := app.ProbeMediaDurationContext(ctx, snap.Target)
	if sess.isCancelled() {
		b.logf("fragment probe cancelled %s", logChatUser(chatID, snap.UserID))
		return
	}
	if err != nil || duration <= 0 {
		if err != nil {
			b.logError(fmt.Sprintf("fragment probe %s", logChatUser(chatID, snap.UserID)), err)
		}
		sess.mutate(func(s *Session) {
			s.MediaDuration = 0
			s.Fragment = nil
		})
		b.askModeWithNotice(chatID, sess, app.FragmentUnavailableText(app.LocaleRU))
		return
	}

	sess.mutate(func(s *Session) {
		s.MediaDuration = duration
	})
	b.askFragmentChoice(chatID, sess)
}

func (b *Bot) askFragmentChoice(chatID int64, sess *Session) {
	snap := sess.snapshot()
	if snap.MediaDuration <= 0 {
		b.askModeWithNotice(chatID, sess, app.FragmentUnavailableText(app.LocaleRU))
		return
	}

	sess.mutate(func(s *Session) {
		s.State = StateAwaitingFragmentChoice
		s.Fragment = nil
	})

	snap = sess.snapshot()
	b.upsertSessionKeyboard(chatID, sess, b.fragmentPromptText(sess), kbFragmentChoice(b.allowURLStartFragment(snap)))
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
	b.upsertSessionMessage(chatID, sess, text, nil)
}

func (b *Bot) upsertSessionKeyboard(chatID int64, sess *Session, text string, kb models.InlineKeyboardMarkup) {
	b.upsertSessionMessage(chatID, sess, text, &kb)
}

func (b *Bot) upsertSessionMessage(chatID int64, sess *Session, text string, kb *models.InlineKeyboardMarkup) {
	snap := sess.snapshot()
	if snap.StatusMsgID == 0 {
		msg, _ := b.sendWithKeyboard(chatID, text, kb)
		if msg.ID != 0 {
			sess.mutate(func(s *Session) {
				s.StatusMsgID = msg.ID
			})
		}
		return
	}
	if kb == nil {
		b.edit(chatID, snap.StatusMsgID, text)
		return
	}
	b.editWithKeyboard(chatID, snap.StatusMsgID, text, kb)
}
