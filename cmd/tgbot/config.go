//go:build bot

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	app "YouTubeBuild/internal/app"
	"YouTubeBuild/internal/bot"
)

const (
	envBotConfigPath        = "VOLREN_BOT_CONFIG"
	envBotToken             = "VOLREN_BOT_TOKEN"
	envBotLocalServer       = "VOLREN_BOT_LOCAL_SERVER"
	envBotAPIURL            = "VOLREN_BOT_API_URL"
	envBotAdminIDs          = "VOLREN_BOT_ADMIN_IDS"
	envBotOwnerIDs          = "VOLREN_BOT_OWNER_IDS"
	envBotPremiumStarsPrice = "VOLREN_BOT_PREMIUM_STARS_PRICE"
	envBotPremiumUsersPath  = "VOLREN_BOT_PREMIUM_USERS_PATH"
	envBotKnownUsersPath    = "VOLREN_BOT_USERS_PATH"
	envBotTimersPath        = "VOLREN_BOT_TIMERS_PATH"
)

type fileBotConfig struct {
	Token             string  `json:"token"`
	LocalServer       *bool   `json:"local_server"`
	ServerURL         string  `json:"server_url"`
	AdminIDs          []int64 `json:"admin_ids"`
	OwnerIDs          []int64 `json:"owner_ids"`
	PremiumStarsPrice int     `json:"premium_stars_price"`
	PremiumUsersPath  string  `json:"premium_users_path"`
	KnownUsersPath    string  `json:"known_users_path"`
	TimersPath        string  `json:"timers_path"`
}

func loadBotBootstrapConfig() (string, bot.Config, error) {
	fileCfg, err := loadBotFileConfig()
	if err != nil {
		return "", bot.Config{}, err
	}

	cfg := bot.Config{
		ServerURL:         strings.TrimSpace(fileCfg.ServerURL),
		LocalServer:       fileBool(fileCfg.LocalServer, true),
		AdminIDs:          idSetFromSlice(fileCfg.AdminIDs),
		OwnerIDs:          idSetFromSlice(fileCfg.OwnerIDs),
		PremiumStarsPrice: fileCfg.PremiumStarsPrice,
		PremiumUsersPath:  strings.TrimSpace(fileCfg.PremiumUsersPath),
		KnownUsersPath:    strings.TrimSpace(fileCfg.KnownUsersPath),
		TimersPath:        strings.TrimSpace(fileCfg.TimersPath),
	}

	token := strings.TrimSpace(fileCfg.Token)
	if envToken := strings.TrimSpace(os.Getenv(envBotToken)); envToken != "" {
		token = envToken
	}
	if value, ok, err := readEnvBool(envBotLocalServer); err != nil {
		return "", bot.Config{}, err
	} else if ok {
		cfg.LocalServer = value
	}
	if value := strings.TrimSpace(os.Getenv(envBotAPIURL)); value != "" {
		cfg.ServerURL = value
	}
	if value := strings.TrimSpace(os.Getenv(envBotPremiumUsersPath)); value != "" {
		cfg.PremiumUsersPath = value
	}
	if value := strings.TrimSpace(os.Getenv(envBotKnownUsersPath)); value != "" {
		cfg.KnownUsersPath = value
	}
	if value := strings.TrimSpace(os.Getenv(envBotTimersPath)); value != "" {
		cfg.TimersPath = value
	}
	if value, ok, err := readEnvInt(envBotPremiumStarsPrice); err != nil {
		return "", bot.Config{}, err
	} else if ok {
		cfg.PremiumStarsPrice = value
	}
	if value := strings.TrimSpace(os.Getenv(envBotAdminIDs)); value != "" {
		ids, err := parseIDSet(value)
		if err != nil {
			return "", bot.Config{}, fmt.Errorf("admins: %w", err)
		}
		cfg.AdminIDs = ids
	}
	if value := strings.TrimSpace(os.Getenv(envBotOwnerIDs)); value != "" {
		ids, err := parseIDSet(value)
		if err != nil {
			return "", bot.Config{}, fmt.Errorf("owners: %w", err)
		}
		cfg.OwnerIDs = ids
	}
	if strings.TrimSpace(token) == "" {
		return "", bot.Config{}, fmt.Errorf("bot token is required; set %s or provide it in %s", envBotToken, defaultBotConfigPath())
	}
	return token, cfg, nil
}

func loadBotFileConfig() (fileBotConfig, error) {
	path, explicit := botConfigPath()
	if path == "" {
		return fileBotConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !explicit && os.IsNotExist(err) {
			return fileBotConfig{}, nil
		}
		return fileBotConfig{}, fmt.Errorf("read bot config %s: %w", path, err)
	}

	var cfg fileBotConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fileBotConfig{}, fmt.Errorf("decode bot config %s: %w", path, err)
	}
	return cfg, nil
}

func botConfigPath() (string, bool) {
	if value := strings.TrimSpace(os.Getenv(envBotConfigPath)); value != "" {
		return filepath.Clean(value), true
	}
	return defaultBotConfigPath(), false
}

func defaultBotConfigPath() string {
	return filepath.Join(app.ConfigDir, "bot.json")
}

func idSetFromSlice(ids []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id != 0 {
			out[id] = struct{}{}
		}
	}
	return out
}

func fileBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func readEnvBool(key string) (bool, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false, false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false, fmt.Errorf("%s: invalid boolean %q", key, raw)
	}
	return value, true, nil
}

func readEnvInt(key string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s: invalid integer %q", key, raw)
	}
	return value, true, nil
}
