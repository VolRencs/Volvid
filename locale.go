package main

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

func nextLocale(l Locale) Locale {
	if l == LocaleEN {
		return LocaleRU
	}
	return LocaleEN
}

const localeFileName = ".volren_locale"

func loadLocale() Locale {
	if AppDir == "" {
		return LocaleEN
	}
	b, err := os.ReadFile(filepath.Join(AppDir, localeFileName))
	if err != nil {
		return LocaleEN
	}
	return parseLocale(string(b))
}

func saveLocale(l Locale) error {
	if AppDir == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(AppDir, localeFileName), []byte(l.String()+"\n"), 0o644)
}
