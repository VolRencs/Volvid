package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	app "YouTubeBuild/internal/app"
	"charm.land/lipgloss/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type binding struct {
	key  string
	help string
}

func (m Model) kbUp() binding       { return binding{key: "↑/k", help: m.u().HelpUp} }
func (m Model) kbDown() binding     { return binding{key: "↓/j", help: m.u().HelpDown} }
func (m Model) kbEnter() binding    { return binding{key: "enter", help: m.u().HelpEnter} }
func (m Model) kbQuit() binding     { return binding{key: "ctrl+c", help: m.u().HelpQuit} }
func (m Model) kbUpdDeps() binding  { return binding{key: "ctrl+u", help: m.u().HelpDeps} }
func (m Model) kbSpace() binding    { return binding{key: "space", help: m.u().HelpSpace} }
func (m Model) kbAll() binding      { return binding{key: "a", help: m.u().HelpAll} }
func (m Model) kbSlash() binding    { return binding{key: "/", help: m.u().HelpSlash} }
func (m Model) kbSearch() binding   { return binding{key: "ctrl+g", help: m.u().HelpSearch} }
func (m Model) kbEsc() binding      { return binding{key: "esc", help: m.u().HelpBack} }
func (m Model) kbAny() binding      { return binding{key: m.u().HelpAnyKey, help: m.u().HelpExit} }
func (m Model) spinnerView() string { return spinnerFrames[m.spinnerFrame%len(spinnerFrames)] }

func (m Model) hint(bindings ...binding) string {
	parts := make([]string, 0, len(bindings))
	for _, item := range bindings {
		parts = append(parts, item.key+" "+item.help)
	}
	return "\n" + sDim.Render(strings.Join(parts, "   "))
}

func (m Model) menuAndNav() string {
	return m.menu.View() + "\n" + m.hint(m.kbUp(), m.kbDown(), m.kbEnter())
}

func (m Model) menuAndNavWithError() string {
	body := m.menuAndNav()
	if m.flowErr == "" {
		return body
	}
	return body + "\n" + renderErrorLine(m.flowErr)
}

func (m Model) renderSpinnerScreen(text string) string {
	return "  " + sTitle.Render(m.spinnerView()) + sDim.Render(text)
}

func (m Model) renderMenuScreen(title, subtitle string) string {
	menuBody := m.menuAndNavWithError()
	contentWidth := max(lipgloss.Width(menuBody), lipgloss.Width(sBold.Render(title)))

	var b strings.Builder
	b.WriteString(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, sBold.Render(title)))
	if subtitle != "" {
		for _, line := range strings.Split(subtitle, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			b.WriteString("\n")
			b.WriteString(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, sDim.Render(line)))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(menuBody)
	return b.String()
}

func (m Model) renderPromptMenu() string {
	u := m.u()

	switch m.screen {
	case scrUpdateReady:
		return sOk.Render(u.UpdateAvail) +
			sBold.Render(m.updateInfo.Latest) +
			sDim.Render(fmt.Sprintf(u.CurrentVerShort, app.Version)) +
			"\n\n" +
			m.menuAndNav()

	default:
		return sWarn.Render(u.PlaylistMixWarn) + "\n\n" + m.menuAndNav()
	}
}

func (m Model) renderUpdateDone() string {
	u := m.u()
	var b strings.Builder

	b.WriteString(sOk.Render(u.UpdateDonePrefix))
	if m.updateInfo != nil {
		b.WriteString(sBold.Render(m.updateInfo.Latest))
	}
	b.WriteString("\n\n")
	if app.IsWindows {
		b.WriteString(sDim.Render(u.UpdateAppliedWin))
	} else {
		b.WriteString(sDim.Render(u.UpdateAppliedUnix))
		b.WriteString(m.hint(m.kbAny()))
	}
	return b.String()
}

func (m Model) renderDepsUpdateScreen() string {
	title := "  Dependencies"
	if m.locale == app.LocaleRU {
		title = "  Зависимости"
	}

	rows := make([]depStatusRow, 0, len(m.deps.Dependencies())+2)
	for _, dep := range m.deps.Dependencies() {
		rows = append(rows, depStatusRow{
			Label: dep.Name,
			Value: m.depLineValue(dep),
		})
	}
	rows = append(rows,
		depStatusRow{Label: "cookies", Value: m.depAccessValue(m.deps.Cookies.Status, m.cookiesAccessDetail())},
		depStatusRow{Label: "js", Value: m.depAccessValue(m.deps.Runtime.Status, m.runtimeAccessDetail())},
	)

	block := m.renderDepStatusRows(rows)

	var b strings.Builder
	b.WriteString(sBold.Render(title))
	b.WriteString("\n\n")
	b.WriteString(block)

	systemCount := 0
	for _, dep := range m.deps.Dependencies() {
		if dep.Source == app.DepSystem {
			systemCount++
		}
	}
	if systemCount > 0 {
		note := "System dependencies are not updated here."
		if m.locale == app.LocaleRU {
			note = "Системные зависимости здесь не обновляются."
		}
		b.WriteString("\n\n" + lipgloss.PlaceHorizontal(lipgloss.Width(block), lipgloss.Center, sDim.Render(note)))
	}
	if m.depErr != "" {
		errLine := sErr.Render(m.u().ErrPrefix) + sDim.Render(m.depErr)
		b.WriteString("\n\n" + lipgloss.PlaceHorizontal(max(lipgloss.Width(block), lipgloss.Width(errLine)), lipgloss.Center, errLine))
	}
	b.WriteString("\n\n")
	b.WriteString(m.menuAndNav())
	return b.String()
}

type depStatusRow struct {
	Label string
	Value string
}

func (m Model) renderDepStatusRows(rows []depStatusRow) string {
	labelWidth := 0
	lines := make([]string, 0, len(rows))

	for _, row := range rows {
		labelWidth = max(labelWidth, lipgloss.Width(strings.TrimSpace(row.Label)))
	}

	for _, row := range rows {
		label := fmt.Sprintf("%*s", labelWidth, strings.TrimSpace(row.Label))
		line := sGray.Render(label) + sDim.Render("  :  ") + row.Value
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) depLineValue(dep app.DependencyInfo) string {
	role := m.depRoleText(dep)
	if !dep.Available {
		return sErr.Render(m.depText("missing")) + sDim.Render("  ["+role+"]")
	}

	meta := []string{string(dep.Source), role}
	version := dep.Version
	if strings.TrimSpace(version) == "" {
		version = m.depText("available")
	}
	return sOk.Render(version) + sDim.Render("  ["+strings.Join(meta, ", ")+"]")
}

func (m Model) depAccessValue(status, detail string) string {
	status = strings.TrimSpace(status)
	detail = strings.TrimSpace(detail)
	switch status {
	case "", "browser not found", "not found":
		return sDim.Render(m.depText("not_active"))
	case "active":
		if detail == "" {
			return sOk.Render(m.depText("active"))
		}
		return sOk.Render(detail) + sDim.Render("  ["+status+"]")
	default:
		if detail == "" {
			return sWarn.Render(status)
		}
		return sWarn.Render(detail) + sDim.Render("  ["+status+"]")
	}
}

func (m Model) depText(kind string) string {
	if m.locale == app.LocaleRU {
		switch kind {
		case "active":
			return "активно"
		case "missing":
			return "не найден"
		case "not_active":
			return "не активно"
		case "available":
			return "доступно"
		}
		return kind
	}
	switch kind {
	case "active":
		return "active"
	case "missing":
		return "missing"
	case "not_active":
		return "not active"
	case "available":
		return "available"
	}
	return kind
}

func (m Model) cookiesAccessDetail() string {
	if strings.TrimSpace(m.deps.Cookies.Browser) == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(m.deps.Cookies.Browser), "firefox") {
		return m.deps.Cookies.Browser
	}
	if profile := strings.TrimSpace(m.deps.Cookies.Profile); profile != "" {
		return m.deps.Cookies.Browser + ":" + filepath.Base(profile)
	}
	return m.deps.Cookies.Browser
}

func (m Model) runtimeAccessDetail() string {
	return strings.TrimSpace(m.deps.Runtime.Name)
}

func (m Model) depRoleText(dep app.DependencyInfo) string {
	if dep.Required {
		if m.locale == app.LocaleRU {
			return "обязательно"
		}
		return "required"
	}
	if m.locale == app.LocaleRU {
		return "опционально"
	}
	return "optional"
}
