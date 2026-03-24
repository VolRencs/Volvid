package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	defaultTelegramServerURL  = "https://api.telegram.org"
	defaultLocalServerURL     = "http://127.0.0.1:8081"
	telegramCloudMaxSendBytes = int64(50 * 1024 * 1024)
	telegramLocalMaxSendBytes = int64(2000 * 1024 * 1024)
	telegramAPITimeout        = 20 * time.Second
	telegramFileSendTimeout   = 10 * time.Minute
)

type Config struct {
	ServerURL   string
	LocalServer bool
	AdminIDs    idSet
	OwnerIDs    idSet
}

func (c Config) normalized() Config {
	c.ServerURL = strings.TrimRight(strings.TrimSpace(c.ServerURL), "/")
	if c.ServerURL == "" {
		if c.LocalServer {
			c.ServerURL = defaultLocalServerURL
		} else {
			c.ServerURL = defaultTelegramServerURL
		}
	}
	return c
}

func (c Config) options() []tg.Option {
	c = c.normalized()
	if c.ServerURL == defaultTelegramServerURL {
		return nil
	}
	return []tg.Option{tg.WithServerURL(c.ServerURL)}
}

func (c Config) needsManualInitCheck() bool {
	c = c.normalized()
	return c.ServerURL != defaultTelegramServerURL
}

func (c Config) backendLabel() string {
	if c.LocalServer {
		return "Local Bot API Server"
	}
	return "Telegram Bot API"
}

func (c Config) maxSendBytes() int64 {
	if c.LocalServer {
		return telegramLocalMaxSendBytes
	}
	return telegramCloudMaxSendBytes
}

func (c Config) sendLimitNotice() string {
	if c.LocalServer {
		return "Local Bot API Server принимает файлы до 2000 МБ."
	}
	return "Telegram Bot API принимает файлы до 50 МБ."
}

func adminCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), telegramAPITimeout)
}

var botAPIHTTPClient = &http.Client{Timeout: telegramAPITimeout}

type botAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

func callNoParamMethod(token string, cfg Config, method string, dest any) error {
	cfg = cfg.normalized()

	ctx, cancel := adminCtx()
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.ServerURL+"/bot"+token+"/"+method, http.NoBody)
	if err != nil {
		return fmt.Errorf("создание запроса %s: %w", method, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := botAPIHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("запрос %s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("чтение ответа %s: %w", method, err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return fmt.Errorf("Bot API вернул пустой ответ для %s", method)
	}

	var apiResp botAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("разбор ответа %s: %w", method, err)
	}
	if !apiResp.OK {
		if apiResp.Description != "" {
			return fmt.Errorf("%s: %s", method, apiResp.Description)
		}
		if apiResp.ErrorCode != 0 {
			return fmt.Errorf("%s: Bot API error %d", method, apiResp.ErrorCode)
		}
		return fmt.Errorf("%s: Bot API вернул ok=false", method)
	}
	if dest != nil && len(apiResp.Result) > 0 {
		if err := json.Unmarshal(apiResp.Result, dest); err != nil {
			return fmt.Errorf("разбор result %s: %w", method, err)
		}
	}
	return nil
}

func probeGetMe(token string, cfg Config) error {
	var me models.User
	return callNoParamMethod(token, cfg, "getMe", &me)
}

func newAdminClient(token string, cfg Config) (*tg.Bot, Config, error) {
	cfg = cfg.normalized()
	opts := cfg.options()
	if cfg.needsManualInitCheck() {
		opts = append(opts, tg.WithSkipGetMe())
	}
	api, err := tg.New(token, opts...)
	if err != nil {
		return nil, cfg, fmt.Errorf("инициализация Bot API клиента: %w", err)
	}
	return api, cfg, nil
}

func LogoutCloud(token string) error {
	return callNoParamMethod(token, Config{}, "logout", nil)
}

func DeleteWebhook(token string, cfg Config, dropPending bool) error {
	api, cfg, err := newAdminClient(token, cfg)
	if err != nil {
		return err
	}

	ctx, cancel := adminCtx()
	defer cancel()

	ok, err := api.DeleteWebhook(ctx, &tg.DeleteWebhookParams{
		DropPendingUpdates: dropPending,
	})
	if err != nil {
		return fmt.Errorf("deleteWebhook %s: %w", cfg.ServerURL, err)
	}
	if !ok {
		return fmt.Errorf("deleteWebhook %s: сервер вернул ok=false", cfg.ServerURL)
	}
	return nil
}

func CloseServer(token string, cfg Config) error {
	cfg = cfg.normalized()
	if err := callNoParamMethod(token, cfg, "close", nil); err != nil {
		return fmt.Errorf("close %s: %w", cfg.ServerURL, err)
	}
	return nil
}
