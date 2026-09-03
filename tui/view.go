package tui

import (
	"fmt"
	"strings"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type noticeKind uint8

const (
	noticeNone noticeKind = iota
	noticeSuccess
	noticeWarn
	noticeError
)

type binding struct {
	key  string
	help string
}

type screenView struct {
	title      string
	subtitle   string
	body       string
	notice     string
	noticeKind noticeKind
	bindings   []binding
}

func (m Model) View() tea.View {
	content := m.buildScreen(m.renderCard(m.screenView()))
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "Volvid · v" + app.Version
	v.Cursor = nil
	return v
}

func (m Model) buildScreen(body string) string {
	topBar := m.renderTopBar()
	footer := m.renderLocaleFooter()
	if m.width == 0 || m.height == 0 {
		return topBar + "\n\n" + body + "\n\n" + footer
	}

	mainH := max(1, m.height-3)
	vertical := lipgloss.Center
	if lipgloss.Height(body) >= mainH {
		vertical = lipgloss.Top
	}
	content := lipgloss.Place(m.width, mainH, lipgloss.Center, vertical, body)
	return topBar + "\n" + content + "\n" + footer
}

func (m Model) renderCard(view screenView) string {
	parts := []string{m.renderHeader(view.title, view.subtitle)}
	if strings.TrimSpace(view.notice) != "" && view.noticeKind != noticeNone {
		parts = append(parts, m.renderNotice(view.notice, view.noticeKind))
	}
	if strings.TrimSpace(view.body) != "" {
		parts = append(parts, strings.Trim(view.body, "\n"))
	}
	if len(view.bindings) > 0 {
		parts = append(parts, m.renderFooterHelp(view.bindings...))
	}
	return m.cardStyle().Width(m.cardWidth()).Render(strings.Join(parts, m.sectionGap()))
}

func (m Model) renderHeader(title, subtitle string) string {
	parts := []string{sAccent.Render("▍ ") + sBold.Render(strings.TrimSpace(title))}
	if subtitle = strings.TrimSpace(subtitle); subtitle != "" {
		parts = append(parts, m.renderSubtitle(subtitle))
	}
	return strings.Join(parts, "\n") + "\n" + sep(m.cardBodyWidth())
}

func (m Model) renderSubtitle(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		lines[i] = sSubtitle.Render(strings.TrimSpace(line))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderTopBar() string {
	left := sBrandMark.Render("◆") + " " + sBold.Render("Volvid") +
		sVersionChip.Render("  v"+app.Version)

	right := m.depBadge()
	if right == "" {
		return left
	}
	if m.width > 0 && lipgloss.Width(left)+2+lipgloss.Width(right) > m.width {
		return left
	}
	if m.width == 0 {
		return left + "  " + right
	}
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) depBadge() string {
	chips := []string{
		renderStatusChip("yt-dlp", versionBadgeValue(m.deps.YTDLP.Version), m.deps.YTDLP.Available),
		renderStatusChip("ffmpeg", versionBadgeValue(m.deps.FFmpeg.Version), m.deps.FFmpeg.Available),
	}
	var action string
	if m.canOpenDependencyScreen() {
		action = renderActionBadge("Ctrl+U", m.u().HelpDeps)
	}

	parts := chips
	if action != "" && m.width >= 72 {
		parts = append(parts, action)
	}
	joined := strings.Join(parts, "  ")

	if m.width == 0 {
		if action != "" {
			return strings.Join(append(chips, action), "  ")
		}
		return joined
	}
	if m.width < 72 {
		if action != "" {
			return action
		}
		return ""
	}
	return joined
}

func (m Model) renderLocaleFooter() string {
	hint := sHelpBracket.Render("[") + sHelpKey.Render("Tab") + sHelpBracket.Render("]") +
		" " + sLocaleChip.Render(strings.ToUpper(m.locale.String()))
	if m.width == 0 {
		return hint
	}
	return lipgloss.Place(m.width, 1, lipgloss.Right, lipgloss.Top, hint)
}

func (m Model) screenView() screenView {
	u := m.u()

	switch m.screen {
	case scrUpdateCheck, scrPlaylistFetch, scrQualityFetch, scrSearchFetch, scrFragmentProbe:
		return screenView{
			title: m.stageTitle(),
			body:  m.renderSpinnerScreen(m.stageTitle()),
		}

	case scrUpdateReady:
		subtitle := strings.TrimSpace(fmt.Sprintf(u.CurrentVerShort, app.Version))
		if m.updateInfo != nil {
			latest := strings.TrimSpace(m.updateInfo.Latest)
			if latest != "" {
				subtitle = latest + "  ·  " + subtitle
			}
		}
		return screenView{
			title:      strings.TrimSpace(u.UpdateAvail),
			subtitle:   subtitle,
			body:       m.menu.View(m.menuWidth()),
			notice:     m.depErr,
			noticeKind: noticeError,
			bindings:   m.menuBindings(),
		}

	case scrUpdateDl, scrDepDl:
		label := m.depLabel
		if m.screen == scrUpdateDl && m.updateInfo != nil {
			label = fmt.Sprintf(u.DepLabelFmt, m.updateInfo.Latest)
		}
		return screenView{
			title:    m.stageTitle(),
			subtitle: strings.TrimSpace(label),
			body:     m.viewDependencyProgress(),
		}

	case scrUpdateDone:
		subtitle := ""
		if m.updateInfo != nil {
			subtitle = m.updateInfo.Latest
		}
		return screenView{
			title:    strings.TrimSpace(u.UpdateDonePrefix),
			subtitle: subtitle,
			body:     m.renderUpdateDone(),
			bindings: []binding{m.kbAny()},
		}

	case scrDepUpdate:
		notice := m.depErr
		kind := noticeError
		if notice == "" && m.depUpdateDone {
			notice = u.DepsOK
			kind = noticeSuccess
		}
		return screenView{
			title:      strings.TrimSpace(u.DepTitle),
			subtitle:   m.depScreenSubtitle(),
			body:       m.viewDepsManage(),
			notice:     notice,
			noticeKind: kind,
			bindings:   m.depBindings(),
		}

	case scrURL:
		return screenView{
			title:      strings.TrimSpace(u.PasteURL),
			subtitle:   strings.TrimSpace(u.URLHints),
			body:       m.viewHome(),
			notice:     m.urlErr,
			noticeKind: noticeError,
			bindings:   []binding{m.kbEnter(), m.kbSearch(), m.kbPickFolder(), m.kbOpenFolder()},
		}

	case scrSearchInput:
		return screenView{
			title:      strings.TrimSpace(u.SearchTitle),
			subtitle:   strings.TrimSpace(u.SearchPrompt),
			body:       renderInputField(m.searchInput),
			notice:     m.searchErr,
			noticeKind: noticeError,
			bindings:   []binding{m.kbEnter(), m.kbEsc()},
		}

	case scrSearchResults:
		return m.choiceScreen(u.SearchTitle, m.searchQuery, "")

	case scrPlaylistAsk:
		return m.choiceScreen(u.ModeTitle, m.url, u.PlaylistMixWarn)

	case scrPlaylist:
		return screenView{
			title:      m.playlistTitle(),
			subtitle:   m.playlistSubtitle(),
			body:       m.viewPlaylist(),
			notice:     m.plInputErr,
			noticeKind: noticeError,
			bindings:   m.playlistBindings(),
		}

	case scrFragmentChoice:
		return m.choiceScreen(u.FragmentTitle, m.fragmentChoiceSubtitle(), m.flowErr)

	case scrFragmentInput:
		return screenView{
			title:      strings.TrimSpace(u.FragmentInputTitle),
			subtitle:   strings.TrimSpace(u.FragmentInputPrompt),
			body:       m.renderInputWithHint(m.fragmentIn, app.FragmentInputHintFor(m.locale, m.mediaDuration)),
			notice:     m.fragmentErr,
			noticeKind: noticeError,
			bindings:   []binding{m.kbEnter(), m.kbEsc()},
		}

	case scrMode:
		return m.choiceScreen(u.ModeTitle, "", m.flowErr)
	case scrAudio:
		return m.choiceScreen(u.AudioTitle, "", m.flowErr)
	case scrQuality:
		return m.choiceScreen(u.QualityTitle, "", m.flowErr)
	case scrVideoOutput:
		return m.choiceScreen(u.VideoOutputTitle, m.profile.Label, m.flowErr)
	case scrWorkers:
		return m.choiceScreen(u.ParallelFmt, fmt.Sprintf(u.WorkersQueuedFmt, len(m.dlEntries)), "")

	case scrDownload:
		return screenView{
			title:    strings.TrimSpace(m.downloadTitle()),
			subtitle: m.downloadSubtitle(),
			body:     m.viewDownload(),
			bindings: []binding{m.kbCancel()},
		}

	case scrSummary:
		notice := ""
		kind := noticeNone
		if text := strings.TrimSpace(m.downloadErr); text != "" {
			notice = text
			kind = noticeError
			if m.dlTotal > 0 && m.dlDone > 0 {
				kind = noticeWarn
			}
		}
		return screenView{
			title:      m.summaryTitle(),
			subtitle:   m.summarySubtitle(),
			body:       m.viewSummary(),
			notice:     notice,
			noticeKind: kind,
			bindings:   m.summaryBindings(),
		}
	}

	return screenView{
		title: "Volvid",
		body:  m.renderSpinnerScreen(m.stageTitle()),
	}
}

func (m Model) choiceScreen(title, subtitle, notice string) screenView {
	kind := noticeWarn
	if notice == "" {
		kind = noticeNone
	}
	return screenView{
		title:      strings.TrimSpace(title),
		subtitle:   strings.TrimSpace(subtitle),
		body:       m.menu.View(m.menuWidth()),
		notice:     notice,
		noticeKind: kind,
		bindings:   m.menuBindings(m.kbEsc()),
	}
}

// ---------- key binding labels ----------

func (m Model) kbMove() binding   { return binding{key: "↑/↓", help: m.u().HelpMove} }
func (m Model) kbDigits() binding { return binding{key: "1-9", help: m.u().HelpDigits} }
func (m Model) kbEnter() binding  { return binding{key: "Enter", help: m.u().HelpEnter} }
func (m Model) kbSpace() binding  { return binding{key: "Space", help: m.u().HelpSpace} }
func (m Model) kbAll() binding    { return binding{key: "A", help: m.u().HelpAll} }
func (m Model) kbSlash() binding  { return binding{key: "/", help: m.u().HelpSlash} }
func (m Model) kbSearch() binding { return binding{key: "Ctrl+G", help: m.u().HelpSearch} }
func (m Model) kbPickFolder() binding {
	return binding{key: "Ctrl+O", help: m.u().HelpPickFolder}
}
func (m Model) kbEsc() binding { return binding{key: "Esc", help: m.u().HelpBack} }
func (m Model) kbCancel() binding {
	return binding{key: "Esc", help: m.u().HelpCancel}
}
func (m Model) kbAny() binding { return binding{key: m.u().HelpAnyKey, help: m.u().HelpExit} }
func (m Model) kbOpenFolder() binding {
	return binding{key: "O", help: m.u().HelpOpenFolder}
}

func (m Model) menuBindings(extra ...binding) []binding {
	bindings := []binding{m.kbMove(), m.kbDigits(), m.kbEnter()}
	return append(bindings, extra...)
}

func (m Model) playlistBindings() []binding {
	if m.plInputMode {
		return []binding{m.kbEnter(), m.kbEsc()}
	}
	return []binding{m.kbMove(), m.kbSpace(), m.kbEnter(), m.kbAll(), m.kbSlash(), m.kbEsc()}
}

func (m Model) summaryBindings() []binding {
	bindings := []binding{m.kbMove(), m.kbEnter()}
	if m.singleOK || m.dlDone > 0 {
		bindings = append(bindings, m.kbOpenFolder())
	}
	return bindings
}

func (m Model) depBindings() []binding {
	if m.depMode == depModeManage {
		return m.menuBindings(m.kbEsc())
	}
	return m.menuBindings()
}

// ---------- chrome pieces ----------

func (m Model) renderInputWithHint(field inputField, hint string) string {
	parts := []string{renderInputField(field)}
	if hint = strings.TrimSpace(hint); hint != "" {
		parts = append(parts, sInputHint.Render(hint))
	}
	return strings.Join(parts, "\n")
}

func (m Model) renderFooterHelp(bindings ...binding) string {
	parts := make([]string, 0, len(bindings))
	for _, item := range bindings {
		parts = append(parts,
			sHelpBracket.Render("[")+sHelpKey.Render(item.key)+sHelpBracket.Render("]")+
				" "+sHelpText.Render(item.help),
		)
	}
	body := joinFittedParts(m.cardBodyWidth(), parts, "  ·  ")
	return sep(m.cardBodyWidth()) + "\n" + body
}

func (m Model) renderNotice(text string, kind noticeKind) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	tag := noticeTag(m.u(), kind)
	switch kind {
	case noticeSuccess:
		return sNoticeSuccess.Render(tag + sBody.Render(text))
	case noticeWarn:
		return sNoticeWarn.Render(tag + sBody.Render(text))
	case noticeError:
		return sNoticeErr.Render(tag + sBody.Render(text))
	default:
		return text
	}
}

func (m Model) stageTitle() string {
	u := m.u()
	switch m.screen {
	case scrDepDl:
		return strings.TrimSpace(u.DepsUpdating)
	case scrUpdateDl:
		return strings.TrimSpace(u.AppUpdating)
	case scrPlaylistFetch:
		return strings.TrimSpace(u.SpinnerPlaylist)
	case scrQualityFetch:
		return strings.TrimSpace(u.SpinnerQuality)
	case scrSearchFetch:
		return strings.TrimSpace(u.SpinnerSearch)
	case scrFragmentProbe:
		return strings.TrimSpace(u.SpinnerFragment)
	default:
		return strings.TrimSpace(u.SpinnerUpdate)
	}
}

func (m Model) renderSpinnerScreen(text string) string {
	return m.renderSectionBlock("", sTitle.Render(m.spinnerView())+"  "+sBody.Render(strings.TrimSpace(text)))
}

func (m Model) fragmentChoiceSubtitle() string {
	u := m.u()
	lines := []string{strings.TrimSpace(u.FragmentHint)}
	if durationText := app.FragmentDurationText(m.locale, m.mediaDuration); durationText != "" {
		lines = append(lines, durationText)
	}
	if m.canUseURLStartFragment() {
		lines = append(lines, fmt.Sprintf(u.FragmentFromURLFmt, app.FormatClockTimestamp(m.target.URLStartAt)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) depScreenSubtitle() string {
	if m.depRefreshing {
		return m.u().DepsRefreshing
	}
	return m.u().DepSubtitle
}

func (m Model) playlistTitle() string {
	if m.plInfo == nil {
		return fmt.Sprintf(m.u().PlVideosFmt, 0)
	}
	return trunc(strings.TrimSpace(m.plInfo.Title), max(1, m.cardBodyWidth()-4))
}

func (m Model) playlistSubtitle() string {
	total := 0
	if m.plInfo != nil {
		total = len(m.plInfo.Entries)
	}
	subtitle := fmt.Sprintf(m.u().PlSelectedFmt, m.selectedPlaylistCount(), total)
	if total > 0 && total > m.playlistViewportHeight() {
		start := m.plTop + 1
		end := min(total, m.plTop+m.playlistViewportHeight())
		subtitle += fmt.Sprintf("  ·  %d-%d/%d", start, end, total)
	}
	return subtitle
}

func (m Model) downloadTitle() string {
	if m.dlTotal > 0 {
		return strings.TrimSpace(fmt.Sprintf(m.u().PlaylistBarFmt, m.dlTotal))
	}
	return m.u().Downloading
}

func (m Model) downloadSubtitle() string {
	if m.dlTotal > 0 {
		queued := max(0, m.dlTotal-(m.dlDone+m.dlFailed))
		return fmt.Sprintf(m.u().QueueFmt, queued) + "  ·  " + formatElapsed(m.dlElapsed)
	}
	return formatElapsed(m.dlElapsed)
}

func (m Model) summaryTitle() string {
	var glyph string
	switch {
	case m.allDownloadFailed():
		glyph = sErr.Render(iconCross)
	case m.partiallyDownloadFailed():
		glyph = sWarn.Render(iconDotOn)
	default:
		glyph = sOk.Render(iconCheck)
	}
	return glyph + "  " + m.summaryOutcome()
}

func (m Model) allDownloadFailed() bool {
	if m.dlTotal > 0 {
		return m.dlDone == 0 && m.dlFailed > 0
	}
	return !m.singleOK
}

func (m Model) partiallyDownloadFailed() bool {
	return m.dlTotal > 0 && m.dlFailed > 0 && m.dlDone > 0
}

func (m Model) summaryOutcome() string {
	u := m.u()
	if m.dlTotal > 0 {
		switch {
		case m.dlDone == 0 && m.dlFailed > 0:
			return u.SummaryFail
		case m.dlFailed > 0:
			return u.SummaryPartial
		default:
			return u.SummaryOK
		}
	}
	if !m.singleOK {
		return u.SummaryFail
	}
	return u.SummaryOK
}

func (m Model) summarySubtitle() string {
	if m.dlTotal > 0 {
		return fmt.Sprintf("%s %d  ·  %s %d  ·  %s",
			sOk.Render(iconDotOn), m.dlDone,
			sErr.Render(iconDotOn), m.dlFailed,
			formatElapsed(m.dlElapsed))
	}
	return formatElapsed(m.dlElapsed)
}
