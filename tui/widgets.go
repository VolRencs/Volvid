package tui

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	app "YouTubeBuild/internal/app"

	"charm.land/lipgloss/v2"
)

// ---------- menu component (state) ----------

type menu struct {
	items  []string
	cursor int
}

func (m *menu) SetItems(items []string) {
	if slices.Equal(m.items, items) {
		return
	}
	m.items = append([]string(nil), items...)
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

// ---------- list rendering (shared by menu and playlist) ----------

type listRowData struct {
	index    string
	label    string
	checked  bool
	hasCheck bool
	active   bool
}

func listLabelWidth(width int, data listRowData) int {
	rowWidth := fitWidth(width, menuW, 24)
	indexWidth := max(2, lipgloss.Width(data.index))
	labelWidth := rowWidth - 2 - indexWidth - 2
	if data.hasCheck {
		labelWidth -= 4
	}
	return max(10, labelWidth)
}

func renderListRow(width int, data listRowData) string {
	indexWidth := max(2, lipgloss.Width(data.index))

	textStyle := sListItemText
	indexStyle := sListIndex.Width(indexWidth)
	style := sListRow
	marker := sListLead.Render("  ")

	if data.active {
		marker = sListLeadAct.Render(iconMarker + " ")
		textStyle = sListItemTextAct
		indexStyle = sListIndexAct.Width(indexWidth)
		style = sListRowAct
	}

	var b strings.Builder
	b.WriteString(marker)
	b.WriteString(indexStyle.Render(fmt.Sprintf("%*s", indexWidth, data.index)))
	b.WriteString("  ")

	if data.hasCheck {
		check := sMeta.Render(iconDotOff)
		if data.checked {
			check = sOk.Render(iconDotOn)
		}
		b.WriteString(check)
		b.WriteString("  ")
	}

	b.WriteString(textStyle.Render(data.label))

	return style.Render(b.String())
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

// ---------- primitives ----------

func renderInputField(field inputField) string {
	style := sInputBox.Width(max(3, field.width+2))
	if field.Focused() {
		style = sInputBoxFocus.Width(max(3, field.width+2))
	}
	return style.Render(field.View())
}

func renderProgressBar(width int, pct float64) string {
	if width <= 0 {
		return ""
	}

	percent := math.Max(0, math.Min(1, pct/100))
	filledWidth := int(math.Round(float64(width) * percent))
	filledWidth = max(0, min(width, filledWidth))

	var b strings.Builder
	if filledWidth > 0 {
		blend := lipgloss.Blend1D(width*2, progressBlendStops...)
		blendIndex := 0
		for range filledWidth {
			b.WriteString(lipgloss.NewStyle().
				Foreground(blend[blendIndex]).
				Background(blend[blendIndex]).
				Render(progressFullChar))
			blendIndex += 2
		}
	}

	if filledWidth < width {
		b.WriteString(sBarRest.Render(strings.Repeat(progressEmptyChar, width-filledWidth)))
	}
	return b.String()
}

func renderBadge(label, value string) string {
	return sBadge.Render(sBadgeLabel.Render(label+":") + " " + sBadgeValue.Render(value))
}

func renderStatusChip(label, value string, ok bool) string {
	dot := cSuccess
	if value == "" || value == "—" || !ok {
		dot = cError
	}
	return sBadge.Render(
		sStatusDot.Foreground(dot).Render(iconDotOn) +
			sBadgeLabel.Render(" "+label+" ") +
			sBadgeValue.Render(value),
	)
}

func renderActionBadge(key, label string) string {
	return sBadge.Render(
		sHelpBracket.Render("[") + sBadgeHotkey.Render(key) + sHelpBracket.Render("]") +
			" " + sMeta.Render(label),
	)
}

func renderFileLink(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return sLink.Render(path)
}

func versionBadgeValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	return value
}

func noticeTag(u *app.UIStrings, kind noticeKind) string {
	switch kind {
	case noticeSuccess:
		return sNoticeTag.Background(cSuccess).Render(u.NoticeSuccess) + " "
	case noticeWarn:
		return sNoticeTag.Background(cWarn).Render(u.NoticeWarn) + " "
	case noticeError:
		return sNoticeTag.Background(cError).Render(u.NoticeError) + " "
	default:
		return ""
	}
}

func (m Model) spinnerView() string {
	return spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
}

func (m Model) renderSectionBlock(title, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	body = strings.Trim(body, "\n")
	w := m.sectionBodyWidth()
	if title = strings.TrimSpace(title); title != "" {
		return sSectionTitle.Render(title) + "\n" + sSectionBox.Width(w).Render(body)
	}
	return sSectionBox.Width(w).Render(body)
}

func (m Model) cardStyle() lipgloss.Style {
	py, px := m.cardPadding()
	return m.screenCardStyle().Padding(py, px)
}

// screenCardStyle tints the card border by flow outcome.
func (m Model) screenCardStyle() lipgloss.Style {
	border := cBorder
	switch {
	case m.screen == scrSummary && m.allDownloadFailed():
		border = lipgloss.Color("#7A4A55")
	case m.screen == scrSummary && m.partiallyDownloadFailed():
		border = lipgloss.Color("#7A6A3E")
	case m.screen == scrSummary:
		border = lipgloss.Color("#2F6B54")
	case m.screen == scrDownload:
		border = lipgloss.Color("#31518A")
	case m.screen == scrUpdateDone:
		border = cSuccess
	}
	return sCard.BorderForeground(border)
}
