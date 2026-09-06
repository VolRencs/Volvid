package tui

import (
	"volvid/internal/core"
)

func (m Model) canUseURLStartFragment() bool {
	return m.target.HasURLStart && m.target.URLStartAt > 0 && m.mediaDuration > 0 && m.target.URLStartAt < m.mediaDuration
}
func (m Model) currentProfile() core.OutputProfile {
	if m.profile.Mode != 0 {
		return m.profile
	}
	return m.api.DefaultProfileForMode(m.mode, m.locale)
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
func (m Model) downloadLabel() string {
	profile := m.currentProfile()
	label := profile.Label
	if label == "" {
		switch profile.Mode {
		case core.ModeAudio:
			label = m.u().ModeAudio
		case core.ModeThumbnail:
			label = m.u().ModeThumbnail
		default:
			label = m.u().ModeVideo
		}
	}
	if profile.Mode == core.ModeThumbnail {
		return label
	}
	if fragment := core.FormatFragmentLabel(m.fragment); fragment != "" {
		return label + " [" + fragment + "]"
	}
	return label
}
