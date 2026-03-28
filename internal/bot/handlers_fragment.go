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
		if !snap.Target.HasURLStart || snap.Target.URLStartAt <= 0 {
			return "В ссылке нет стартового таймкода."
		}
		fragment := app.DownloadFragment{StartAt: snap.Target.URLStartAt}
		sess.mutate(func(s *Session) {
			s.Fragment = &fragment
			s.StatusMsgID = msgID
		})
		b.askMode(chatID, sess)
		return ""
	case cbFragmentInput:
		sess.mutate(func(s *Session) {
			s.State = StateAwaitingFragmentInput
			s.StatusMsgID = msgID
		})
		b.edit(chatID, msgID, fragmentInputHint)
		return ""
	default:
		return ""
	}
}

func (b *Bot) handleFragmentInput(chatID int64, sess *Session, raw string) {
	fragment, err := app.ParseFragmentRange(raw)
	if err != nil || !fragment.IsValid() {
		b.send(chatID, "⚠️ Неверный диапазон.\n"+fragmentInputHint)
		return
	}
	sess.mutate(func(s *Session) {
		s.Fragment = &fragment
	})
	b.askMode(chatID, sess)
}
