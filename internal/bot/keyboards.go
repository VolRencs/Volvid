package bot

import (
	app "YouTubeBuild/internal/app"
	"github.com/go-telegram/bot/models"
)

const (
	cbSearchPrefix  = "search:"
	cbPlVideo       = "pl:video"
	cbPlChoose      = "pl:full"
	cbPlTogglePref  = "pl:toggle:"
	cbPlPagePref    = "pl:page:"
	cbPlSelectAll   = "pl:sel:all"
	cbPlSelectNone  = "pl:sel:none"
	cbPlSelectDone  = "pl:sel:done"
	cbModeVideo     = "mode:video"
	cbModeAudio     = "mode:audio"
	cbModeThumb     = "mode:thumb"
	cbAudioPrefix   = "audio:"
	cbQualityPrefix = "q:"
	cbPremiumBuy    = "premium:buy"
	cbNoop          = "action:noop"
	cbCancel        = "action:cancel"
)

func kbPlaylistChoice() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				kbButton("🎬 Только это видео", cbPlVideo),
				kbButton("🎯 Выбрать видео", cbPlChoose),
			},
			{
				kbButton("❌ Отмена", cbCancel),
			},
		},
	}
}

func kbQuality(choices []app.QualityChoice) models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(choices)+1)
	for _, choice := range choices {
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton(choice.Label(app.LocaleRU), cbQualityPrefix+choice.Key),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		kbButton("❌ Отмена", cbCancel),
	})
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func kbMode() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				kbButton("🎬 Видео", cbModeVideo),
				kbButton("🎵 Аудио", cbModeAudio),
			},
			{
				kbButton("🖼 Превью", cbModeThumb),
			},
			{
				kbButton("❌ Отмена", cbCancel),
			},
		},
	}
}

func kbAudioProfiles(profiles []app.OutputProfile) models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(profiles)+1)
	for _, profile := range profiles {
		rows = append(rows, []models.InlineKeyboardButton{
			kbButton(profile.Label, cbAudioPrefix+profile.Key),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		kbButton("❌ Отмена", cbCancel),
	})
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func emptyInlineKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}}
}

func kbButton(text, data string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{
		Text:         text,
		CallbackData: data,
	}
}
