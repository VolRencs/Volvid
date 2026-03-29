package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) renderTopBar() string {
	u := m.u()
	badge := " " +
		sGray.Render("yt-dlp ") + verOrDash(m.deps.YTDLP.Version) +
		sDim.Render("  ·  ") +
		sGray.Render("ffmpeg ") + verOrDash(m.deps.FFmpeg.Version) +
		" "

	var action string
	if m.isAppUpdateScreen() {
		action = ""
	} else if m.screen == scrDepDl && m.depErr == "" {
		action = " " + sWarn.Render(m.spinnerView()+u.TopBarDepsBusy) + " "
	} else {
		action = " " + sGray.Render(u.TopBarDeps) + "  " + sDim.Render("Ctrl+U") + " "
	}

	if m.width == 0 {
		return badge + action
	}

	gap := m.width - lipgloss.Width(badge) - lipgloss.Width(action)
	if gap < 1 {
		gap = 1
	}
	return badge + strings.Repeat(" ", gap) + action
}

func (m Model) renderLocaleFooter() string {
	hint := m.u().LangTab + "  " + strings.ToUpper(m.locale.String())
	if m.width == 0 {
		return sDim.Render(hint)
	}
	return lipgloss.Place(m.width, 1, lipgloss.Right, lipgloss.Top, sDim.Render(hint))
}

func (m Model) buildScreen(body string) string {
	topBar := m.renderTopBar()
	footer := m.renderLocaleFooter()
	if m.width == 0 || m.height == 0 {
		return topBar + "\n" + body + "\n" + footer
	}

	mainH := max(1, m.height-2)
	return topBar + "\n" + lipgloss.Place(m.width, mainH, lipgloss.Center, lipgloss.Center, body) + "\n" + footer
}
