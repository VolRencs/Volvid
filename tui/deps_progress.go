package tui

import (
	"fmt"
	"strings"
	"volvid/internal/i18n"
)

func (m Model) viewDependencyProgress() string {
	lines := []string{renderProgressBar(m.progressBarWidth(), m.depProgress.Pct)}
	var meta strings.Builder
	meta.WriteString(sOk.Render(fmt.Sprintf("%.1f%%", m.depProgress.Pct)))
	if m.depProgress.DoneB > 0 {
		meta.WriteString("  " + sValue.Render(i18n.FormatBytes(m.depProgress.DoneB, m.locale)))
		if m.depProgress.TotalB > 0 {
			meta.WriteString(sMeta.Render(" / " + i18n.FormatBytes(m.depProgress.TotalB, m.locale)))
		}
		if m.depProgress.Speed != "" {
			meta.WriteString("  " + sTitle.Render(m.depProgress.Speed))
		}
	}
	lines = append(lines, meta.String())
	return m.renderSectionBlock("", strings.Join(lines, "\n"))
}
func (m Model) renderUpdateDone() string {
	if m.api.IsWindows() {
		return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedWin))
	}
	return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedUnix))
}
