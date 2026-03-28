package bot

import (
	"strings"

	app "YouTubeBuild/internal/app"
)

const playlistSelectionPageSize = 5

func playlistSelectionPageBounds(info *app.PlaylistInfo, page int) (normalized, start, end, pageCount int) {
	if info == nil || len(info.Entries) == 0 {
		return 0, 0, 0, 1
	}

	pageCount = (len(info.Entries) + playlistSelectionPageSize - 1) / playlistSelectionPageSize
	normalized = max(0, min(page, pageCount-1))
	start = normalized * playlistSelectionPageSize
	end = min(len(info.Entries), start+playlistSelectionPageSize)
	return normalized, start, end, pageCount
}

func truncateButtonLabel(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
}
