package tui

import (
	"fmt"
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

	case scrFFmpegAsk:
		return sWarn.Render(u.FFmpegWarn) +
			"\n" +
			sDim.Render(u.FFmpegHint) +
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
	u := m.u()
	var b strings.Builder

	if m.depUpdateDone {
		b.WriteString(sOk.Render(u.DepsOK) + "\n\n")
		b.WriteString("  " + sGray.Render("yt-dlp  ") + sOk.Render(m.ytdlpVer) + "\n")
		if m.ffmpegVer != "" {
			b.WriteString("  " + sGray.Render("ffmpeg  ") + sOk.Render(m.ffmpegVer) + "\n")
		}
		b.WriteString(m.hint(m.kbEnter()))
		return b.String()
	}

	if m.depErr != "" {
		b.WriteString(sErr.Render(u.ErrPrefix) + sDim.Render(m.depErr) + "\n")
		b.WriteString(m.hint(m.kbEnter()))
		return b.String()
	}

	b.WriteString(m.viewDependencyProgress(u.DepsUpdating))
	return b.String()
}
