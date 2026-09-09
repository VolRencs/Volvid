package tui

import (
	"charm.land/lipgloss/v2"
	"fmt"
	"strconv"
	"strings"
	"volvid/internal/core"
)

func (m Model) viewHome() string {
	parts := []string{
		m.viewHomeInput(),
		m.viewDownloadsLocation(),
	}
	if section := m.viewHomeSession(); section != "" {
		parts = append(parts, section)
	}
	return strings.Join(compactSections(parts...), m.sectionGap())
}

func (m Model) viewHomeInput() string {
	title := sSectionTitle.Render(m.u().HomeInputTitle)
	return title + "\n" + renderInputField(m.urlInput)
}

func (m Model) viewDownloadsLocation() string {
	pathWidth := max(18, m.cardBodyWidth()-10)
	body := renderFileLink(trunc(m.api.DownloadsDir(), pathWidth))
	return m.renderSectionBlock(m.u().HomeOutputTitle, body)
}

func (m Model) viewHomeSession() string {
	if m.compactHomeLayout() && len(m.session.Items) == 0 {
		return ""
	}
	return m.sessionBlock(3)
}

// sessionBlock renders the shared session history section (limit<=0 = all).
func (m Model) sessionBlock(limit int) string {
	stats := renderStatusChip(m.u().HomeStatSuccess, strconv.Itoa(m.session.Success), true) + "  " +
		renderStatusChip(m.u().HomeStatFailed, strconv.Itoa(m.session.Failed), false)
	if len(m.session.Items) == 0 {
		return m.renderSectionBlock(m.u().HomeSessionTitle, stats+"\n"+sMeta.Render(m.u().HomeSessionEmpty))
	}

	items := m.session.Items
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}

	width := max(18, m.cardBodyWidth()/2)
	rows := make([]string, 0, len(items)+1)
	rows = append(rows, stats)
	for _, item := range items {
		icon := statusIcon(item.OK)
		rows = append(rows,
			icon+"  "+sValue.Render(trunc(item.Label, width))+"\n"+
				sMeta.Render(trunc(item.URL, width+14)),
		)
	}
	return m.renderSectionBlock(m.u().HomeSessionTitle, strings.Join(rows, "\n\n"))
}

func (m Model) viewPlaylist() string {
	if m.plInfo == nil {
		return ""
	}

	var parts []string
	parts = append(parts, m.renderSectionBlock("", m.renderPlaylistItems()))
	if m.plInputMode {
		parts = append(parts,
			m.renderSectionBlock(m.u().PlEnterNums, renderInputField(m.plInput)),
		)
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) renderPlaylistItems() string {
	entries := m.playlistEntries()
	start := m.plTop
	end := min(len(entries), start+m.playlistViewportHeight())
	indexWidth := max(2, len(strconv.Itoa(len(entries))))
	rowWidth := m.cardBodyWidth()
	staticWidth := lipgloss.Width("  ") + lipgloss.Width(iconDotOn) + indexWidth + 14
	titleWidth := max(1, min(m.playlistTitleWidth(), rowWidth-staticWidth))

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		entry := entries[i]
		duration := fmt.Sprintf("%8s", core.FormatDuration(entry.Duration))
		data := listRowData{
			index:    fmt.Sprintf("%*d", indexWidth, entry.Index),
			hasCheck: true,
			checked:  m.plSelected[entry.Index],
			active:   i == m.plCursor,
		}
		labelWidth := listLabelWidth(rowWidth, data) - lipgloss.Width(duration) - 2
		title := sPlTitle.Width(max(1, titleWidth)).Render(trunc(entry.Title, max(1, min(titleWidth, labelWidth))))
		data.label = title + "  " + sTableMeta.Render(duration)

		lines = append(lines, renderListRow(rowWidth, data))
	}
	return strings.Join(lines, "\n")
}

func (m Model) subtitleSubtitle() string {
	total := len(m.subTracks)
	subtitle := fmt.Sprintf(m.u().PlSelectedFmt, m.selectedSubtitleCount(), total)
	rows := total + 1
	if rows > 0 && rows > m.playlistViewportHeight() {
		start := m.subTop + 1
		end := min(rows, m.subTop+m.playlistViewportHeight())
		subtitle += fmt.Sprintf("  ·  %d-%d/%d", start, end, rows)
	}
	return subtitle
}

func (m Model) viewSubtitles() string {
	if len(m.subTracks) == 0 {
		return ""
	}
	return m.renderSectionBlock("", m.renderSubtitleItems())
}

func (m Model) renderSubtitleItems() string {
	tracks := m.subTracks
	rows := len(tracks) + 1
	start := m.subTop
	end := min(rows, start+m.playlistViewportHeight())
	indexWidth := max(2, len(strconv.Itoa(rows)))
	rowWidth := m.cardBodyWidth()

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		if i == 0 {
			data := listRowData{
				index:  "",
				active: m.subCursor == 0,
				label:  trunc(m.u().SubtitleOff, listLabelWidth(rowWidth, listRowData{index: ""})),
			}
			lines = append(lines, renderListRow(rowWidth, data))
			continue
		}
		track := tracks[i-1]
		data := listRowData{
			index:    fmt.Sprintf("%*d", indexWidth, i),
			hasCheck: true,
			checked:  m.subSelected[track.Lang],
			active:   i == m.subCursor,
		}
		data.label = trunc(m.subtitleTrackLabel(track), listLabelWidth(rowWidth, data))
		lines = append(lines, renderListRow(rowWidth, data))
	}
	return strings.Join(lines, "\n")
}

func (m Model) audioTrackSubtitle() string {
	total := len(m.audioTracks)
	subtitle := fmt.Sprintf(m.u().PlSelectedFmt, m.selectedAudioCount(), total)
	rows := total + 1
	if rows > 0 && rows > m.playlistViewportHeight() {
		start := m.audioTop + 1
		end := min(rows, m.audioTop+m.playlistViewportHeight())
		subtitle += fmt.Sprintf("  ·  %d-%d/%d", start, end, rows)
	}
	return subtitle
}

func (m Model) viewAudioTracks() string {
	if len(m.audioTracks) == 0 {
		return ""
	}
	return m.renderSectionBlock("", m.renderAudioTrackItems())
}

func (m Model) renderAudioTrackItems() string {
	tracks := m.audioTracks
	rows := len(tracks) + 1
	start := m.audioTop
	end := min(rows, start+m.playlistViewportHeight())
	indexWidth := max(2, len(strconv.Itoa(rows)))
	rowWidth := m.cardBodyWidth()

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		if i == 0 {
			data := listRowData{
				index:  "",
				active: m.audioCursor == 0,
				label:  trunc(m.u().AudioTrackOriginal, listLabelWidth(rowWidth, listRowData{index: ""})),
			}
			lines = append(lines, renderListRow(rowWidth, data))
			continue
		}
		track := tracks[i-1]
		data := listRowData{
			index:    fmt.Sprintf("%*d", indexWidth, i),
			hasCheck: true,
			checked:  m.audioSelected[track.Lang],
			active:   i == m.audioCursor,
		}
		data.label = trunc(strings.TrimSpace(track.Lang), listLabelWidth(rowWidth, data))
		lines = append(lines, renderListRow(rowWidth, data))
	}
	return strings.Join(lines, "\n")
}
