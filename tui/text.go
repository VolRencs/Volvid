package tui

import (
	"fmt"
	"strings"
	"time"

	app "YouTubeBuild/internal/app"

	"charm.land/lipgloss/v2"
)

func trunc(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	if lipgloss.Width(value) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}

	var b strings.Builder
	width := 0
	for _, r := range value {
		rw := lipgloss.Width(string(r))
		if width+rw > limit-1 {
			break
		}
		b.WriteRune(r)
		width += rw
	}

	out := strings.TrimRight(b.String(), " ")
	if out == "" {
		return "…"
	}
	return out + "…"
}

func sep(width int) string {
	return sRule.Render(strings.Repeat("─", width))
}

func compactSections(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}

func joinFittedParts(width int, parts []string, sep string) string {
	parts = compactSections(parts...)
	if len(parts) == 0 {
		return ""
	}
	if width <= 0 {
		return strings.Join(parts, sep)
	}

	lines := make([]string, 0, len(parts))
	current := parts[0]
	for _, part := range parts[1:] {
		candidate := current + sep + part
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = part
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func fmtStats(l app.Locale, done, total int64, speed string) string {
	switch {
	case total > 0:
		return sValue.Render(app.FmtBytesFor(done, l)) + sDim.Render("/"+app.FmtBytesFor(total, l)) + speedSuffix(speed)
	case done > 0:
		return sValue.Render(app.FmtBytesFor(done, l)) + speedSuffix(speed)
	default:
		return sDim.Render("…")
	}
}

func speedSuffix(speed string) string {
	speed = strings.TrimSpace(speed)
	if speed == "" {
		return ""
	}
	return "  " + sTitle.Render(speed)
}

func statusIcon(ok bool) string {
	if ok {
		return sOk.Render(iconDotOn)
	}
	return sErr.Render(iconDotOn)
}
