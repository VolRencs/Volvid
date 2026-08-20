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
	b, err := os.ReadFile(localePath())
	if err != nil {
		return LocaleEN
	}
	return parseLocale(string(b))
}

func SaveLocale(l Locale) error {
	return writeAppConfig(localePath(), l.String()+"\n")
}
