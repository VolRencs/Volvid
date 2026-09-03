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

const localeFileName = ".volvid_locale"

func localePath(env *Env) string {
	return filepath.Join(env.ConfigDir, localeFileName)
}

func LoadLocale(env *Env) Locale {
	b, err := os.ReadFile(localePath(env))
	if err != nil {
		return LocaleEN
	}
	return parseLocale(string(b))
}

func SaveLocale(env *Env, l Locale) error {
	return writeAppConfig(localePath(env), l.String()+"\n")
}
