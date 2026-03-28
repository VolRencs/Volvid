package bot

import (
	"fmt"
	"strings"
)

func progressBar(done, total int) string {
	const width = 10
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := done * width / total
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}

func verLine(name, ver string) string {
	if ver == "" {
		return fmt.Sprintf("• %s — <b>не найден</b>", escapeHTML(name))
	}
	return fmt.Sprintf("• %s — <code>%s</code>", escapeHTML(name), escapeHTML(ver))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
