package tui

import (
	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) gotoChecks() (tea.Model, tea.Cmd) {
	deps := app.DetectDeps()
	m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
	if deps.YtdlpVer == "" {
		m.depLabel = "yt-dlp"
		m.depProgress = app.FileProgress{}
		m.depErr = ""
		m.screen = scrDepDl

		var cmd tea.Cmd
		m.depCh, cmd = launchProgress(func(ch chan<- app.FileProgress) error {
			return app.InstallYtDlpFor(m.locale, ch)
		}, false)
		return m, cmd
	}
	return m.gotoURLWithDeps(deps)
}

func (m Model) gotoURL() (tea.Model, tea.Cmd) {
	return m.gotoURLWithDeps(app.DetectDeps())
}

func (m Model) gotoURLWithDeps(deps app.CheckDepsResult) (tea.Model, tea.Cmd) {
	m.ytdlpVer, m.ffmpegVer = deps.YtdlpVer, deps.FFmpegVer
	if app.IsWindows && deps.FFmpegMissing {
		m.screen = scrFFmpegAsk
		m = m.syncMenu()
		return m, nil
	}
	m.screen = scrURL
	return m, m.urlInput.Focus()
}

func (m Model) startDepUpdate() (tea.Model, tea.Cmd) {
	m.prevScreen = m.screen
	m.depProgress = app.FileProgress{}
	m.depErr = ""
	m.depUpdateDone = false
	m.screen = scrDepUpdate

	var cmd tea.Cmd
	m.depCh, cmd = launchProgress(func(ch chan<- app.FileProgress) error {
		return app.InstallAllDepsFor(m.locale, ch)
	}, false)
	return m, cmd
}
