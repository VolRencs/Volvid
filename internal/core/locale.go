// Package core holds pure domain types and functions.
//
// No I/O, no processes, no network, no UI strings here — only data,
// parsing and policies. Everything in core is unit-testable without
// any infrastructure. Use-cases (internal/services) and adapters
// (internal/adapters) build on top of these types.
package core

import "strings"

// Locale is the UI language. Kept in core (not i18n) because domain
// requests carry it (DownloadRequest.Locale) and parsing it has no
// dependency on translated strings.
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

// ParseLocale maps free-form input ("ru", "rus", ...) to a Locale.
func ParseLocale(s string) Locale {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ru", "rus":
		return LocaleRU
	default:
		return LocaleEN
	}
}

// NextLocale toggles between the supported UI languages.
func NextLocale(l Locale) Locale {
	if l == LocaleEN {
		return LocaleRU
	}
	return LocaleEN
}
