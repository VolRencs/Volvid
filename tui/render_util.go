package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	app "YouTubeBuild/internal/app"
	"charm.land/lipgloss/v2"
)

func renderProgressBar(width int, pct float64) string {
	if width <= 0 {
		return ""
	}

	totalWidth := width
	percent := math.Max(0, math.Min(1, pct/100))
	filledWidth := int(math.Round(float64(totalWidth) * percent))
	filledWidth = max(0, min(totalWidth, filledWidth))

	var b strings.Builder
	if filledWidth > 0 {
		blend := lipgloss.Blend1D(totalWidth*2, progressBlendStops...)
		blendIndex := 0
		for range filledWidth {
			b.WriteString(lipgloss.NewStyle().
				Foreground(blend[blendIndex]).
				Background(blend[blendIndex+1]).
				Render(progressFullChar))
			blendIndex += 2
		}
	}

	if filledWidth < totalWidth {
		b.WriteString(sBarRest.Render(strings.Repeat(progressEmptyChar, totalWidth-filledWidth)))
	}
	return b.String()
}

func verOrDash(value string) string {
	if value != "" {
		return sOk.Render(value)
	}
	return sDim.Render("—")
}

func sep(width int) string {
	return sDim.Render(strings.Repeat("─", width))
}

func trunc(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func fmtStats(l app.Locale, done, total int64, speed string) string {
	var stat string
	switch {
	case total > 0:
		stat = sBold.Render(app.FmtBytesFor(done, l)) + sDim.Render("/"+app.FmtBytesFor(total, l))
	case done > 0:
		stat = sBold.Render(app.FmtBytesFor(done, l))
	default:
		stat = sDim.Render("…")
	}
	if speed != "" {
		stat += "  " + sTitle.Render(speed)
	}
	return stat
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
