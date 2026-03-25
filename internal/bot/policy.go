package bot

import (
	"fmt"

	app "YouTubeBuild/internal/app"
)

const (
	regularDownloadSizeLimitBytes = 500 * 1024 * 1024
	premiumDownloadSizeLimitBytes = 2 * 1024 * 1024 * 1024

	regularPlaylistItemLimit = 5
	premiumPlaylistItemLimit = 30
)

func (b *Bot) hasPremium(userID int64) bool {
	return b != nil && b.premium != nil && b.premium.HasPremium(userID)
}

func (b *Bot) downloadSizeLimit(userID int64) int64 {
	if b.hasPremium(userID) {
		return premiumDownloadSizeLimitBytes
	}
	return regularDownloadSizeLimitBytes
}

func (b *Bot) downloadSizeLimitText(userID int64) string {
	return app.FmtBytesFor(b.downloadSizeLimit(userID), app.LocaleRU)
}

func (b *Bot) playlistItemLimit(userID int64) int {
	if b.hasPremium(userID) {
		return premiumPlaylistItemLimit
	}
	return regularPlaylistItemLimit
}

func (b *Bot) playlistItemLimitText(userID int64) string {
	return fmt.Sprintf("%d видео", b.playlistItemLimit(userID))
}

func (b *Bot) validatePlaylistSelectionCount(userID int64, count int) bool {
	return count >= 0 && count <= b.playlistItemLimit(userID)
}

func (b *Bot) downloadSizeLimitExceededText(userID int64) string {
	if b.hasPremium(userID) {
		return "Лимит 2GB превышен."
	}
	return "Лимит 500MB превышен. Купите премиум для увеличения до 2GB"
}

func (b *Bot) playlistItemLimitExceededText(userID int64) string {
	if b.hasPremium(userID) {
		return "Можно выбрать не больше 30 видео из плейлиста."
	}
	return "Можно выбрать не больше 5 видео из плейлиста. Купите премиум для увеличения до 30 видео."
}

func (b *Bot) playlistItemLimitAlert(userID int64) string {
	if b.hasPremium(userID) {
		return "Доступно максимум 30 видео."
	}
	return "Доступно максимум 5 видео. Купите премиум для увеличения до 30."
}

func (b *Bot) notifyDownloadSizeLimitExceeded(chatID int64, msgID int, userID int64) {
	b.notifyEntitlementLimit(chatID, msgID, userID, b.downloadSizeLimitExceededText(userID))
}

func (b *Bot) notifyPlaylistItemLimitExceeded(chatID int64, userID int64) {
	b.notifyEntitlementLimit(chatID, 0, userID, b.playlistItemLimitExceededText(userID))
}

func (b *Bot) notifyEntitlementLimit(chatID int64, msgID int, userID int64, text string) {
	if msgID != 0 {
		b.removeKb(chatID, msgID)
		b.edit(chatID, msgID, text)
	} else {
		b.send(chatID, text)
	}
	if !b.hasPremium(userID) {
		b.sendKb(chatID, text, kbPremiumOffer(b.cfg.PremiumStarsPrice))
	}
}
