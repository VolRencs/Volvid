package bot

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	app "YouTubeBuild/internal/app"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type documentInput struct {
	file        models.InputFile
	displayPath string
	close       func()
}

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
	limit := b.sendLimitBytes()
	if fi.Size() > limit {
		b.logf("sendFile rejected chat=%d path=%s size=%d limit=%d", chatID, path, fi.Size(), limit)
		return errTelegramFileTooLarge
	}

	input, err := b.documentInput(path)
	if err != nil {
		return err
	}
	defer input.closeFile()

	err = normalizeSendFileError(input.displayPath, b.sendDocument(chatID, input.file))
	if err != nil {
		b.logError(fmt.Sprintf("sendFile chat=%d path=%s", chatID, input.displayPath), err)
	}
	return err
}

func (b *Bot) documentInput(path string) (documentInput, error) {
	if b.cfg.LocalServer {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return documentInput{}, fmt.Errorf("abs %s: %w", path, err)
		}
		return documentInput{
			file:        &models.InputFileString{Data: localFileURI(absPath)},
			displayPath: absPath,
		}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return documentInput{}, fmt.Errorf("open %s: %w", path, err)
	}
	return documentInput{
		file: &models.InputFileUpload{
			Filename: filepath.Base(path),
			Data:     f,
		},
		displayPath: path,
		close: func() {
			_ = f.Close()
		},
	}, nil
}

func (b *Bot) sendDocument(chatID int64, file models.InputFile) error {
	ctx, cancel := b.fileSendCtx()
	defer cancel()

	_, err := b.api.SendDocument(ctx, &tg.SendDocumentParams{
		ChatID:   chatID,
		Document: file,
	})
	return err
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

func normalizeSendFileError(path string, err error) error {
	switch {
	case err == nil:
		return nil
	case isTelegramTooLarge(err):
		return errTelegramFileTooLarge
	case isTimeoutError(err):
		return fmt.Errorf("send file %s: timeout waiting for telegram response: %w", path, err)
	default:
		return fmt.Errorf("send file %s: %w", path, err)
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func (d documentInput) closeFile() {
	if d.close != nil {
		d.close()
	}
}
