package core

import (
	"strconv"
	"strings"
)

// Unified tolerant decoders for yt-dlp JSON payloads.
//
// yt-dlp emits numbers as float64, int or numeric strings (sometimes with
// a "%" suffix in progress lines, sometimes null). Single source of truth
// for all adapters.
func decodeString(v any) string {
	s, _ := v.(string)
	return s
}

func decodeStringOr(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

// MapString reads m[key] tolerantly (playlist search results, flat entries).
func MapString(m map[string]any, key, def string) string {
	if m == nil {
		return def
	}
	return decodeStringOr(m[key], def)
}

func decodeFloat(v any) float64 {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

func decodeInt(v any) int64 {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

// MapFloat reads m[key] tolerantly.
func MapFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	return decodeFloat(m[key])
}

// parseIntOrZero parses CLI/progress integers ("12", " 7 ") -> 0 on error.
func ParseIntOrZero(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parsePercentOrZero parses progress percents ("12.5%") -> 0 on error.
func ParsePercentOrZero(raw string) float64 {
	rest, _ := strings.CutSuffix(raw, "%")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return 0
	}
	n, err := strconv.ParseFloat(rest, 64)
	if err != nil {
		return 0
	}
	return n
}
