package tui

import (
	"time"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleDlUpdate(u app.DlUpdate) (tea.Model, tea.Cmd) {
	if u.Type == app.EvClosed {
		return m, nil
	}

	if u.Slot < len(m.slots) {
		s := &m.slots[u.Slot]
		switch u.Type {
		case app.EvStart:
			*s = slotState{title: trunc(u.Text, 50)}
		case app.EvDest:
			s.title = trunc(u.Text, 50)
		case app.EvProgress:
			s.pct = u.Pct
			s.doneB = u.DoneB
			s.totalB = u.TotalB
			s.speed = u.Speed
			s.proc = false
		case app.EvProc, app.EvFallback:
			s.proc = true
			s.label = u.Text
		case app.EvReset:
			*s = slotState{}
		case app.EvDone:
			s.done = u.OK
			s.failed = !u.OK
			s.proc = false
			s.label = trunc(u.ErrText, 64)
			s.pct = 100
		}
	}

	if u.Type != app.EvDone {
		return m, listenDownloadCmd(m.dlCh)
	}

	if u.OK {
		m.dlDone++
	} else {
		m.dlFailed++
		if m.downloadErr == "" {
			m.downloadErr = u.ErrText
		}
	}

	if m.dlTotal == 0 || m.dlDone+m.dlFailed >= m.dlTotal {
		if m.dlTotal == 0 {
			m.singleOK = u.OK
		}
		label := m.downloadLabel()
		if m.dlTotal > 0 {
			label += app.PlaylistSuffix(m.locale, m.dlTotal)
		}
		m.session.Record(label, m.url, m.dlFailed == 0 || (m.dlTotal == 0 && u.OK))
		m.screen = scrSummary
		m.timerActive = false
		m.dlElapsed = time.Since(m.dlStartedAt).Round(time.Second)
		m = m.syncMenu()
		return m, nil
	}

	return m, listenDownloadCmd(m.dlCh)
}

func (m Model) downloadLabel() string {
	profile := m.currentProfile()
	label := profile.Label
	if label == "" {
		switch profile.Mode {
		case app.ModeAudio:
			label = m.u().ModeAudio
		case app.ModeThumbnail:
			label = m.u().ModeThumbnail
		default:
			label = m.u().ModeVideo
		}
	}
	if fragment := m.fragmentLabel(profile.Mode); fragment != "" {
		return label + " [" + fragment + "]"
	}
	return label
}

func (m Model) fragmentLabel(mode app.DownloadMode) string {
	if mode == app.ModeThumbnail {
		return ""
	}
	return app.FormatFragmentLabel(m.fragment)
}

func (m Model) startModeSelection() (tea.Model, tea.Cmd) {
	return m.startModeSelectionWithNotice("")
}

func (m Model) startModeSelectionWithNotice(notice string) (tea.Model, tea.Cmd) {
	m.mode = app.DefaultDownloadMode()
	m.profile = app.DefaultVideoProfile(m.locale)
	m.flowErr = notice
	m.qualityChoices = nil
	m.audioProfiles = nil
	m.screen = scrMode
	m = m.syncMenu()
	return m, nil
}

func (m Model) startQualityScan() (tea.Model, tea.Cmd) {
	m.qualityChoices = nil
	m.profile = app.OutputProfile{}
	m.flowErr = ""
	m.screen = scrQualityFetch
	return m, loadQualityChoicesCmd(m.qualityScanURLs())
}

func (m Model) qualityScanURLs() []string {
	if m.forceSingle || m.plInfo == nil || len(m.dlEntries) == 0 {
		return []string{m.target.DownloadURL(m.forceSingle)}
	}

	urls := make([]string, 0, len(m.dlEntries))
	for _, entry := range m.dlEntries {
		urls = append(urls, entry.URL)
	}
	return urls
}

func (m Model) currentProfile() app.OutputProfile {
	if m.profile.Mode != 0 {
		return m.profile
	}
	return app.DefaultProfileForMode(m.mode, m.locale)
}

func (m Model) startDownload() (tea.Model, tea.Cmd) {
	deps := app.DetectDeps()
	m = m.withDeps(deps)

	switch {
	case !deps.YTDLP.Available:
		m.prevScreen = m.screen
		return m.openDependencyScreenWithError(depModeManage, m.depRequirementText(deps.YTDLP.Name))
	case (m.currentProfile().Mode == app.ModeAudio || m.fragment != nil) && !deps.FFmpeg.Available:
		m.prevScreen = m.screen
		return m.openDependencyScreenWithError(depModeManage, m.depRequirementText(deps.FFmpeg.Name))
	}

	req, err := app.PrepareDownloadRequest(app.DownloadRequest{
		Target:        m.target,
		Profile:       m.currentProfile(),
		Fragment:      m.fragment,
		MediaDuration: m.mediaDuration,
		ForceSingle:   m.forceSingle,
		PlaylistInfo:  m.plInfo,
		Entries:       m.dlEntries,
		Workers:       max(m.numWorkers, 1),
		OutputDir:     app.DlDir,
		Locale:        m.locale,
	})
	if err != nil {
		m.flowErr = err.Error()
		m.restoreDownloadConfigScreen()
		m = m.syncMenu()
		return m, nil
	}

	workers := max(m.numWorkers, 1)
	if len(m.dlEntries) == 0 {
		workers = 1
	}

	m.slots = make([]slotState, workers)
	m.dlDone = 0
	m.dlFailed = 0
	m.dlTotal = len(m.dlEntries)
	m.singleOK = false
	m.downloadErr = ""
	m.dlStartedAt = time.Now()
	m.dlElapsed = 0
	m.timerActive = true

	ch := make(chan app.DlUpdate, 256)
	m.dlCh = ch
	m.screen = scrDownload

	req.Workers = workers
	app.StartDownloadRequest(req, ch)
	return m, tea.Batch(listenDownloadCmd(ch), timerTickCmd())
}

func (m *Model) restoreDownloadConfigScreen() {
	switch m.currentProfile().Mode {
	case app.ModeAudio:
		if m.profile.Mode == 0 {
			m.screen = scrAudio
		}
	case app.ModeThumbnail:
		m.screen = scrMode
	default:
		if m.profile.Mode == 0 {
			m.screen = scrQuality
		}
	}
}
