package tui

import (
	"context"
	"fmt"
	"strings"
	"volvid/internal/core"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

type depStatusRow struct {
	Label string
	Value string
}

type depState string

const (
	depStateActive    depState = "active"
	depStateMissing   depState = "missing"
	depStateNotActive depState = "not_active"
	depStateAvailable depState = "available"
	depStateChecking  depState = "checking"
)

type depScreenMode uint8

const (
	depModeStartup depScreenMode = iota + 1
	depModeManage
)

type depAction struct {
	Label string
	Run   func(Model) (tea.Model, tea.Cmd)
}

func (m Model) depActions() []depAction {
	u := m.u()
	actions := make([]depAction, 0, 5)
	for _, dep := range m.deps.ActionableDependencies() {
		key := dep.Key
		label := fmt.Sprintf(u.DepActionDownloadFmt, dep.Name)
		if dep.Available && dep.Source == core.DepManaged {
			label = fmt.Sprintf(u.DepActionUpdateFmt, dep.Name)
		}
		actions = append(actions, depAction{
			Label: label,
			Run: func(m Model) (tea.Model, tea.Cmd) {
				return m.startDependencyDownload(scrDepDl, key, false, func(ctx context.Context, ch chan<- core.FileProgress) error {
					return m.api.InstallDependency(ctx, key, m.locale, ch)
				})
			},
		})
	}

	if !m.depRefreshing {
		actions = append(actions, depAction{
			Label: u.DepActionRefresh,
			Run:   func(m Model) (tea.Model, tea.Cmd) { return m.startDepsRefresh() },
		})
	}
	if m.depMode == depModeStartup && !m.deps.MissingRequired() {
		actions = append(actions, depAction{
			Label: u.DepActionContinue,
			Run:   func(m Model) (tea.Model, tea.Cmd) { return m.gotoURLWithDeps(m.deps) },
		})
	}
	if m.depMode == depModeManage {
		actions = append(actions, depAction{
			Label: u.DepActionBack,
			Run:   func(m Model) (tea.Model, tea.Cmd) { return m.returnFromDependencyScreen() },
		})
	} else if m.deps.MissingRequired() {
		actions = append(actions, depAction{
			Label: u.DepActionExit,
			Run:   func(m Model) (tea.Model, tea.Cmd) { return m, tea.Quit },
		})
	}
	return actions
}

func (m Model) depRequirementText(name string) string {
	return fmt.Sprintf(m.u().DepRequirementFmt, name)
}

func (m Model) viewDependencyProgress() string {
	lines := []string{renderProgressBar(m.progressBarWidth(), m.depProgress.Pct)}
	meta := progressMeta(m.locale, m.depProgress.Pct, m.depProgress.DoneB, m.depProgress.TotalB, m.depProgress.Speed)
	lines = append(lines, meta)
	return m.renderSectionBlock("", strings.Join(lines, "\n"))
}

func (m Model) renderUpdateDone() string {
	return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedUnix))
}

func (m Model) viewDepsManage() string {
	cookiesDetail := strings.TrimSpace(m.deps.Cookies.Browser)
	if profile := strings.TrimSpace(m.deps.Cookies.ProfileName); cookiesDetail != "" && profile != "" {
		cookiesDetail += ":" + profile
	}

	rows := make([]depStatusRow, 0, len(m.deps.Dependencies())+2)
	for _, dep := range m.deps.Dependencies() {
		rows = append(rows, depStatusRow{Label: dep.Name, Value: m.depLineValue(dep)})
	}
	rows = append(rows,
		depStatusRow{Label: "cookies", Value: m.depAccessValue(m.deps.Cookies.Status, cookiesDetail)},
		depStatusRow{Label: "js", Value: m.depAccessValue(m.deps.Runtime.Status, strings.TrimSpace(m.deps.Runtime.Name))},
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
		if dep.Source == core.DepSystem {
			count++
		}
	}
	return count
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

func (m Model) depLineValue(dep core.DependencyInfo) string {
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
	case "", core.StatusNotFound:
		return sDim.Render(m.depText(depStateNotActive))
	case core.StatusActive:
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
	return ""
}

func (m Model) depRoleText(dep core.DependencyInfo) string {
	if dep.Required {
		return m.u().DepRoleRequired
	}
	return m.u().DepRoleOptional
}

func (m Model) depSourceText(source core.DependencySource) string {
	switch source {
	case core.DepManaged:
		return m.u().DepSourceBundled
	case core.DepSystem:
		return m.u().DepSourceSystem
	default:
		return string(source)
	}
}
