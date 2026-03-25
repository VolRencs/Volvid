package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	app "YouTubeBuild/internal/app"
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
	ServerURL         string
	LocalServer       bool
	AdminIDs          idSet
	OwnerIDs          idSet
	PremiumUsersPath  string
	KnownUsersPath    string
	TimersPath        string
	PremiumStarsPrice int
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
	if strings.TrimSpace(c.PremiumUsersPath) == "" {
		c.PremiumUsersPath = filepath.Join(app.AppDir, "premium_users.json")
	}
	if strings.TrimSpace(c.KnownUsersPath) == "" {
		c.KnownUsersPath = filepath.Join(app.AppDir, "bot_users.json")
	}
	if strings.TrimSpace(c.TimersPath) == "" {
		c.TimersPath = filepath.Join(app.AppDir, "bot_timers.json")
	}
	if c.PremiumStarsPrice <= 0 {
		c.PremiumStarsPrice = 250
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

var botAPIHTTPClient = app.NewHTTPClient(telegramAPITimeout)

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

	req, err := newBotAPIRequest(ctx, cfg, token, method)
	if err != nil {
		return err
	}

	resp, err := botAPIHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("запрос %s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := readBotAPIBody(resp, method)
	if err != nil {
		return err
	}

	return decodeBotAPIResponse(method, body, dest)
}

func newBotAPIRequest(ctx context.Context, cfg Config, token, method string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, botAPIMethodURL(cfg, token, method), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("создание запроса %s: %w", method, err)
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func botAPIMethodURL(cfg Config, token, method string) string {
	cfg = cfg.normalized()
	return cfg.ServerURL + "/bot" + token + "/" + method
}

func readBotAPIBody(resp *http.Response, method string) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("чтение ответа %s: %w", method, err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("Bot API вернул пустой ответ для %s", method)
	}
	return body, nil
}

func decodeBotAPIResponse(method string, body []byte, dest any) error {
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
