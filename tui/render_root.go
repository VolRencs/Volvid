package tui

import (
	"fmt"
	"strings"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) View() tea.View {
	body := m.renderBody()
	screen := m.buildScreen(body)

	v := tea.NewView(screen)
	v.AltScreen = true
	v.WindowTitle = "VolRen Downloader · v" + app.Version
	v.Cursor = nil
	return v
}

func (m Model) renderBody() string {
	u := m.u()
	var b strings.Builder

	header := sHeader.Render(
		sTitle.Render("VolRen") +
			sDim.Render(u.AppSubtitle) +
			"\n" +
			sDim.Render(fmt.Sprintf(u.HeaderPowered, app.Version)),
	)
	b.WriteString(header)
	b.WriteString("\n\n")

	switch m.screen {
	case scrUpdateCheck, scrPlaylistFetch, scrQualityFetch, scrSearchFetch:
		msg := u.SpinnerUpdate
		if m.screen == scrPlaylistFetch {
			msg = u.SpinnerPlaylist
		} else if m.screen == scrQualityFetch {
			msg = u.SpinnerQuality
		} else if m.screen == scrSearchFetch {
			msg = u.SpinnerSearch
		}
		b.WriteString(m.renderSpinnerScreen(msg))

	case scrUpdateReady, scrFFmpegAsk, scrPlaylistAsk:
		b.WriteString(m.renderPromptMenu())

	case scrMode:
		b.WriteString(m.renderMenuScreen(u.ModeTitle, ""))

	case scrAudio:
		b.WriteString(m.renderMenuScreen(u.AudioTitle, ""))

	case scrFragmentChoice:
		subtitle := u.FragmentHint
		if m.target.HasURLStart && m.target.URLStartAt > 0 {
			subtitle += "\n" + fmt.Sprintf(u.FragmentFromURLFmt, app.FormatClockTimestamp(m.target.URLStartAt))
		}
		b.WriteString(m.renderMenuScreen(u.FragmentTitle, subtitle))

	case scrUpdateDl, scrDepDl:
		label := m.depLabel
		if m.screen == scrUpdateDl && m.updateInfo != nil {
			label = fmt.Sprintf(u.DepLabelFmt, m.updateInfo.Latest)
		}
		b.WriteString(m.viewDependencyProgress(label))

	case scrUpdateDone:
		b.WriteString(m.renderUpdateDone())

	case scrDepUpdate:
		b.WriteString(m.renderDepsUpdateScreen())

	case scrURL:
		b.WriteString(m.viewURLScreen())

	case scrSearchInput:
		b.WriteString(m.viewSearchInput())

	case scrFragmentInput:
		b.WriteString(m.viewFragmentInput())

	case scrSearchResults:
		b.WriteString(m.viewSearchResults())

	case scrPlaylist:
		b.WriteString(m.viewPlaylist())

	case scrWorkers, scrQuality:
		if m.screen == scrWorkers {
			b.WriteString(m.renderMenuScreen(u.ParallelFmt, fmt.Sprintf(u.WorkersQueuedFmt, len(m.dlEntries))))
		} else {
			b.WriteString(m.renderMenuScreen(u.QualityTitle, ""))
		}

	case scrDownload:
		b.WriteString(m.viewDownload())

	case scrSummary:
		b.WriteString(m.viewSummary())
	}

	return b.String()
}
