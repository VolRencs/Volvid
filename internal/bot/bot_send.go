package bot

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	app "YouTubeBuild/internal/app"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) sendLimitBytes() int64 {
	if b == nil || b.maxSend <= 0 {
		return telegramCloudMaxSendBytes
	}
	return b.maxSend
}

func (b *Bot) sendLimitText() string {
	return app.FmtBytesFor(b.sendLimitBytes(), app.LocaleRU)
}

func (b *Bot) sendLimitNotice() string {
	return b.cfg.sendLimitNotice()
}

func (b *Bot) sendFile(chatID int64, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if fi.Size() > b.sendLimitBytes() {
		b.logf("sendFile rejected chat=%d path=%s size=%d limit=%d", chatID, path, fi.Size(), b.sendLimitBytes())
		return errTelegramFileTooLarge
	}

	var sendErr error
	if b.cfg.LocalServer {
		sendErr = b.sendFileByLocalPath(chatID, path)
	} else {
		sendErr = b.sendFileByUpload(chatID, path)
		if isTelegramTooLarge(sendErr) {
			return errTelegramFileTooLarge
		}
	}
	if sendErr != nil {
		log.Printf("sendFile chat=%d path=%s: %v", chatID, path, sendErr)
	}
	return sendErr
}

func (b *Bot) sendFileByLocalPath(chatID int64, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs %s: %w", path, err)
	}

	ctx, cancel := b.fileSendCtx()
	defer cancel()

	_, err = b.api.SendDocument(ctx, &tg.SendDocumentParams{
		ChatID:   chatID,
		Document: &models.InputFileString{Data: localFileURI(absPath)},
	})
	if err != nil && isTelegramTooLarge(err) {
		return errTelegramFileTooLarge
	}
	if err != nil {
		return fmt.Errorf("send local file %s: %w", absPath, err)
	}
	return nil
}

func (b *Bot) sendFileByUpload(chatID int64, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	ctx, cancel := b.fileSendCtx()
	defer cancel()

	_, err = b.api.SendDocument(ctx, &tg.SendDocumentParams{
		ChatID: chatID,
		Document: &models.InputFileUpload{
			Filename: filepath.Base(path),
			Data:     f,
		},
	})
	if err != nil {
		return fmt.Errorf("upload file %s: %w", path, err)
	}
	return nil
}

func localFileURI(path string) string {
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func isTelegramTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "413") || strings.Contains(msg, "request entity too large")
}
