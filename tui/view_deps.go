package tui

import (
	"fmt"
	"strings"

	app "YouTubeBuild/internal/app"

	"charm.land/lipgloss/v2"
)

func (m Model) viewDependencyProgress() string {
	lines := []string{renderProgressBar(m.progressBarWidth(), m.depProgress.Pct)}
	var meta strings.Builder
	meta.WriteString(sOk.Render(fmt.Sprintf("%.1f%%", m.depProgress.Pct)))
	if m.depProgress.DoneB > 0 {
		meta.WriteString("  " + sValue.Render(app.FmtBytesFor(m.depProgress.DoneB, m.locale)))
		if m.depProgress.TotalB > 0 {
			meta.WriteString(sMeta.Render(" / " + app.FmtBytesFor(m.depProgress.TotalB, m.locale)))
		}
		if m.depProgress.Speed != "" {
			meta.WriteString("  " + sTitle.Render(m.depProgress.Speed))
		}
	}
	lines = append(lines, meta.String())
	return m.renderSectionBlock("", strings.Join(lines, "\n"))
}

func (m Model) renderUpdateDone() string {
	if m.env.IsWindows {
		return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedWin))
	}
	return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedUnix))
}

func (m Model) viewDepsManage() string {
	rows := make([]depStatusRow, 0, len(m.deps.Dependencies())+2)
	for _, dep := range m.deps.Dependencies() {
		rows = append(rows, depStatusRow{Label: dep.Name, Value: m.depLineValue(dep)})
	}
	rows = append(rows,
		depStatusRow{Label: "cookies", Value: m.depAccessValue(m.deps.Cookies.Status, m.cookiesAccessDetail())},
		depStatusRow{Label: "js", Value: m.depAccessValue(m.deps.Runtime.Status, m.runtimeAccessDetail())},
	)

	parts := []string{m.renderSectionBlock("", renderDepStatusRows(rows))}
	if systemCount := m.systemDepsCount(); systemCount > 0 {
		parts = append(parts, m.renderSectionBlock("", sMeta.Render(m.u().DepSystemNote)))
	}
	if len(m.menu.items) > 0 {
		parts = append(parts, m.menu.View(m.menuWidth()))
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) systemDepsCount() int {
	count := 0
	for _, dep := range m.deps.Dependencies() {
		if dep.Source == app.DepSystem {
			count++
		}
	}
	return count
}

type depStatusRow struct {
	Label string
	Value string
}

func renderDepStatusRows(rows []depStatusRow) string {
	labelWidth := 0
	for _, row := range rows {
		labelWidth = max(labelWidth, lipgloss.Width(strings.TrimSpace(row.Label)))
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		label := fmt.Sprintf("%-*s", labelWidth, strings.TrimSpace(row.Label))
		lines = append(lines, sTableLabel.Render(label)+sMeta.Render("  ·  ")+sTableMeta.Render(row.Value))
	}
	return strings.Join(lines, "\n")
}

func (m Model) depLineValue(dep app.DependencyInfo) string {
	role := m.depRoleText(dep)
	if !dep.Available {
		return sErr.Render(m.depText(depStateMissing)) + sDim.Render("  ["+role+"]")
	}

	meta := []string{m.depSourceText(dep.Source), role}
	version := dep.Version
	checking := strings.TrimSpace(version) == "" && m.depRefreshing
	if checking {
		version = m.depText(depStateChecking)
	}
	if strings.TrimSpace(version) == "" {
		version = m.depText(depStateAvailable)
	}
	if checking {
		return sDim.Render(version) + sDim.Render("  ["+strings.Join(meta, ", ")+"]")
	}
	return sOk.Render(version) + sDim.Render("  ["+strings.Join(meta, ", ")+"]")
}

func (m Model) depAccessValue(status, detail string) string {
	status = strings.TrimSpace(status)
	detail = strings.TrimSpace(detail)
	if m.depRefreshing && status == "" {
		return sDim.Render(m.depText(depStateChecking))
	}
	switch status {
	case "", "browser not found", "not found":
		return sDim.Render(m.depText(depStateNotActive))
	case "active":
		if detail == "" {
			return sOk.Render(m.depText(depStateActive))
		}
		return sOk.Render(detail) + sDim.Render("  ["+status+"]")
	default:
		if detail == "" {
			return sWarn.Render(status)
		}
		return sWarn.Render(detail) + sDim.Render("  ["+status+"]")
	}
}

type depState string

const (
	depStateActive    depState = "active"
	depStateMissing   depState = "missing"
	depStateNotActive depState = "not_active"
	depStateAvailable depState = "available"
	depStateChecking  depState = "checking"
)

func (m Model) depText(kind depState) string {
	u := m.u()
	switch kind {
	case depStateActive:
		return u.DepStatusActive
	case depStateMissing:
		return u.DepStatusMissing
	case depStateNotActive:
		return u.DepStatusNotActive
	case depStateAvailable:
		return u.DepStatusAvailable
	case depStateChecking:
		return u.DepStatusChecking
	}
	return string(kind)
}

func (m Model) cookiesAccessDetail() string {
	browser := strings.TrimSpace(m.deps.Cookies.Browser)
	if browser == "" {
		return ""
	}
	if profile := strings.TrimSpace(m.deps.Cookies.ProfileName); profile != "" {
		return browser + ":" + profile
	}
	return browser
}

func (m Model) runtimeAccessDetail() string {
	return strings.TrimSpace(m.deps.Runtime.Name)
}

func (m Model) depRoleText(dep app.DependencyInfo) string {
	if dep.Required {
		return m.u().DepRoleRequired
	}
	return m.u().DepRoleOptional
}

func (m Model) depSourceText(source app.DependencySource) string {
	switch source {
	case app.DepManaged:
		return m.u().DepSourceBundled
	case app.DepSystem:
		return m.u().DepSourceSystem
	default:
		return string(source)
	}
}
