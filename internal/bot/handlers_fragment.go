package bot

import app "YouTubeBuild/internal/app"

func (b *Bot) handleFragmentChoiceCallback(chatID int64, msgID int, sess *Session, data string) string {
	snap := sess.snapshot()
	if snap.PlInfo != nil && !snap.ForceSingle {
		return "Фрагменты доступны только для одиночной загрузки."
	}

	switch data {
	case cbFragmentAll:
		sess.mutate(func(s *Session) {
			s.Fragment = nil
			s.StatusMsgID = msgID
		})
		b.askMode(chatID, sess)
		return ""
	case cbFragmentURL:
		if snap.MediaDuration <= 0 {
			return app.FragmentUnavailableText(app.LocaleRU)
		}
		if !snap.Target.HasURLStart || snap.Target.URLStartAt <= 0 {
			return "В ссылке нет стартового таймкода."
		}
		fragment := app.DownloadFragment{StartAt: snap.Target.URLStartAt}
		if err := app.ValidateFragmentDuration(fragment, snap.MediaDuration); err != nil {
			return app.FragmentURLStartOutOfBoundsText(app.LocaleRU, snap.MediaDuration)
		}
		sess.mutate(func(s *Session) {
			s.Fragment = &fragment
			s.StatusMsgID = msgID
		})
		b.askMode(chatID, sess)
		return ""
	case cbFragmentInput:
		if snap.MediaDuration <= 0 {
			return app.FragmentUnavailableText(app.LocaleRU)
		}
		sess.mutate(func(s *Session) {
			s.State = StateAwaitingFragmentInput
			s.StatusMsgID = msgID
		})
		b.edit(chatID, msgID, b.fragmentInputText(snap.MediaDuration))
		return ""
	default:
		return ""
	}
}

func (b *Bot) handleFragmentInput(chatID int64, sess *Session, raw string) {
	snap := sess.snapshot()
	fragment, err := app.ParseBoundedFragmentRange(raw, snap.MediaDuration)
	if err != nil {
		b.send(chatID, "⚠️ "+escapeHTML(app.FragmentInputErrorText(app.LocaleRU, err, snap.MediaDuration))+"\n\n"+b.fragmentInputText(snap.MediaDuration))
		return
	}
	sess.mutate(func(s *Session) {
		s.Fragment = &fragment
	})
	b.askMode(chatID, sess)
}
