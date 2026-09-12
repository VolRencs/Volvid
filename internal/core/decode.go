package core

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Unified tolerant decoders for yt-dlp JSON payloads.
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
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint, uint32, uint64:
		return float64(decodeInt(v))
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return f
		}
		return 0
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
	case int32:
		return int64(n)
	case int64:
		return n
	case uint:
		return int64(n)
	case uint32:
		return int64(n)
	case uint64:
		if n > 1<<63-1 {
			return 0
		}
		return int64(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
		if f, err := n.Float64(); err == nil {
			return int64(f)
		}
		return 0
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
	return decodeFloat(m[key])
}

// ParseIntOrZero parses CLI/progress integers ("12", " 7 ") -> 0 on error.
func ParseIntOrZero(raw string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ParsePercentOrZero parses progress percents ("12.5%") -> 0 on error.
func ParsePercentOrZero(raw string) float64 {
	rest, _ := strings.CutSuffix(raw, "%")
	n, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
	if err != nil {
		return 0
	}
	return n
}
