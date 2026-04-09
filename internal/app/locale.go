package app

import (
	"os"
	"path/filepath"
	"strings"
)

type Locale uint8

const (
	LocaleEN Locale = iota
	LocaleRU
)

func (l Locale) String() string {
	if l == LocaleRU {
		return "ru"
	}
	return "en"
}

func parseLocale(s string) Locale {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ru", "rus":
		return LocaleRU
	default:
		return LocaleEN
	}
}

func NextLocale(l Locale) Locale {
	if l == LocaleEN {
		return LocaleRU
	}
	return LocaleEN
}

const localeFileName = ".volren_locale"

func localePath() string {
	return filepath.Join(ConfigDir, localeFileName)
}

func LoadLocale() Locale {
	path := localePath()
	if path == "" {
		return LocaleEN
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return LocaleEN
	}
	return parseLocale(string(b))
}

func SaveLocale(l Locale) error {
	path := localePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(l.String()+"\n"), 0o644)
}
