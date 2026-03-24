package bot

import "github.com/go-telegram/bot/models"

type idSet map[int64]struct{}

func (s idSet) has(id int64) bool {
	if len(s) == 0 {
		return false
	}
	_, ok := s[id]
	return ok
}

func senderID(msg *models.Message) int64 {
	if msg == nil {
		return 0
	}
	if msg.From != nil {
		return msg.From.ID
	}
	if msg.Chat.ID != 0 {
		return msg.Chat.ID
	}
	return 0
}
