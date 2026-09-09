package tui

import (
	"strings"
)

func (m Model) viewDependencyProgress() string {
	lines := []string{renderProgressBar(m.progressBarWidth(), m.depProgress.Pct)}
	meta := progressMeta(m.locale, m.depProgress.Pct, m.depProgress.DoneB, m.depProgress.TotalB, m.depProgress.Speed)
	lines = append(lines, meta)
	return m.renderSectionBlock("", strings.Join(lines, "\n"))
}
func (m Model) renderUpdateDone() string {
	if m.api.IsWindows() {
		return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedWin))
	}
	return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedUnix))
}
