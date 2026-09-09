package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

// Menu component: state (items+cursor) and item sources live together.
// Rendering helper View() stays with the component; generic list-row
// primitives (listRowData/renderListRow) remain in widgets.go.
type menu struct {
	items  []string
	cursor int
}

func (m *menu) SetItems(items []string) {
	if slices.Equal(m.items, items) {
		return
	}
	m.items = slices.Clone(items)
	m.cursor = 0
}

func (m *menu) SetCursor(index int) {
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	m.cursor = max(0, min(index, len(m.items)-1))
}

func (m *menu) Move(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.cursor = max(0, min(m.cursor+delta, len(m.items)-1))
}

func (m menu) Index() int {
	if len(m.items) == 0 {
		return 0
	}
	return m.cursor
}

func (m menu) View(width int) string {
	lines := make([]string, len(m.items))
	for i, item := range m.items {
		data := listRowData{
			index:  strconv.Itoa(i + 1),
			label:  item,
			active: i == m.cursor,
		}
		data.label = trunc(item, listLabelWidth(width, data))
		lines[i] = renderListRow(width, data)
	}
	return strings.Join(lines, "\n")
}

// Item sources: one place that maps screen -> []string.
func (m Model) menuItems() []string {
	u := m.u()
	switch m.screen {
	case scrUpdateReady:
		return []string{u.MenuUpdateY, u.MenuUpdateN}
	case scrPlaylistAsk:
		return []string{u.MenuVidOnly, u.MenuOpenPl}
	case scrDepUpdate:
		actions := m.depActions()
		items := make([]string, 0, len(actions))
		for _, action := range actions {
			items = append(items, action.Label)
		}
		return items
	case scrMode:
		return m.modeOptions()
	case scrFragmentChoice:
		return m.fragmentChoiceOptions()
	case scrAudio:
		return m.audioOptions()
	case scrSearchResults:
		return m.searchResultOptions()
	case scrSummary:
		return []string{u.MenuAgainY, u.MenuAgainN}
	case scrQuality:
		return m.qualityOptions()
	case scrVideoOutput:
		return m.videoOutputOptions()
	case scrWorkers:
		return m.workerMenuOptions(min(len(m.dlEntries), 5))
	default:
		return nil
	}
}

func (m Model) syncMenu() Model {
	m.menuDigits = ""
	m.menu.SetItems(m.menuItems())
	return m
}

func (m Model) gotoScreen(s screen) Model {
	m.screen = s
	return m.syncMenu()
}

func (m Model) qualityOptions() []string {
	return i18n.QualityChoiceLabels(m.qualityChoices, m.locale)
}

func (m Model) audioOptions() []string {
	return core.OutputProfileLabels(m.audioProfiles)
}

func (m Model) videoOutputOptions() []string {
	return core.OutputProfileLabels(m.videoProfiles)
}

func (m Model) subtitleTrackLabel(track core.SubtitleTrack) string {
	label := strings.TrimSpace(track.Lang)
	if track.Auto {
		label += " (" + m.u().SubtitleAutoTag + ")"
	}
	return label
}

func (m Model) modeOptions() []string {
	u := m.u()
	return []string{u.ModeVideo, u.ModeAudio, u.ModeThumbnail}
}

func (m Model) workerMenuOptions(n int) []string {
	u := m.u()
	if n <= 0 {
		return nil
	}
	opts := make([]string, n)
	opts[0] = u.WorkerSeq
	for i := 1; i < n; i++ {
		opts[i] = fmt.Sprintf(u.WorkerNFmt, i+1)
	}
	return opts
}

func (m Model) fragmentChoiceOptions() []string {
	u := m.u()
	options := []string{u.MenuFullVideo}
	if m.canUseURLStartFragment() {
		options = append(options, fmt.Sprintf("%s (%s)", u.MenuFromURLStart, core.FormatClockTimestamp(m.target.URLStartAt)))
	}
	return append(options, u.MenuManualRange)
}

func (m Model) searchResultOptions() []string {
	options := make([]string, 0, len(m.searchResults))
	for i, result := range m.searchResults {
		label := strings.TrimSpace(result.Title)
		if label == "" {
			label = fmt.Sprintf(m.u().VideoTitleFmt, i+1)
		}
		if result.Duration > 0 {
			label += "  ·  " + core.FormatDuration(result.Duration)
		}
		options = append(options, label)
	}
	return options
}
