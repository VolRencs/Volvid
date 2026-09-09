package tui

import (
	"fmt"
	"strings"
	"volvid/internal/core"
	"volvid/internal/i18n"

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
	v.WindowTitle = "Volvid · v" + m.api.AppVersion()
	v.Cursor = nil
	return v
}
func (m Model) buildScreen(body string) string {
	topBar := m.renderTopBar()
	footer := m.renderLocaleFooter()
	if m.width == 0 || m.height == 0 {
		return topBar + "\n\n" + body + "\n\n" + footer
	}

	mainH := max(1, m.height-topBarReservedHeight)
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
		sVersionChip.Render("  v"+m.api.AppVersion())

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
	if m.depRefreshing {
		chips = append(chips, renderActionBadge("…", m.u().DepsRefreshing))
	}
	var action string
	if m.canOpenDependencyScreen() {
		action = renderActionBadge("Ctrl+U", m.u().HelpDeps)
	}

	parts := chips
	if action != "" && m.width >= breakDepsWideWidth {
		parts = append(parts, action)
	}
	joined := strings.Join(parts, "  ")

	if m.width == 0 {
		if action != "" {
			return strings.Join(append(chips, action), "  ")
		}
		return joined
	}
	if m.width < breakDepsWideWidth {
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
	case scrUpdateCheck, scrPlaylistFetch, scrQualityFetch, scrSearchFetch, scrFragmentProbe, scrAudioTrackFetch, scrSubsFetch:
		return m.loadingScreen(m.stageTitle())

	case scrUpdateReady:
		subtitle := strings.TrimSpace(fmt.Sprintf(u.CurrentVerShort, m.api.AppVersion()))
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
		return m.inputScreen(
			strings.TrimSpace(u.SearchTitle),
			strings.TrimSpace(u.SearchPrompt),
			renderInputField(m.searchInput),
			m.searchErr,
		)

	case scrSearchResults:
		return m.menuScreen(u.SearchTitle, m.searchQuery, "")

	case scrPlaylistAsk:
		return m.menuScreen(u.ModeTitle, m.url, u.PlaylistMixWarn)

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
		return m.menuScreen(u.FragmentTitle, m.fragmentChoiceSubtitle(), m.flowErr)

	case scrFragmentInput:
		return m.inputScreen(
			strings.TrimSpace(u.FragmentInputTitle),
			strings.TrimSpace(u.FragmentInputPrompt),
			m.renderInputWithHint(m.fragmentIn, i18n.FragmentInputHintFor(m.locale, m.mediaDuration)),
			m.fragmentErr,
		)

	case scrMode:
		return m.menuScreen(u.ModeTitle, "", m.flowErr)
	case scrAudio:
		return m.menuScreen(u.AudioTitle, "", m.flowErr)
	case scrQuality:
		return m.menuScreen(u.QualityTitle, "", m.flowErr)
	case scrVideoOutput:
		return m.menuScreen(u.VideoOutputTitle, m.profile.Label, m.flowErr)
	case scrAudioTrack:
		return screenView{
			title:      strings.TrimSpace(u.AudioTrackTitle),
			subtitle:   m.audioTrackSubtitle(),
			body:       m.viewAudioTracks(),
			notice:     m.flowErr,
			noticeKind: noticeWarn,
			bindings:   []binding{m.kbMove(), m.kbSpace(), m.kbEnter(), m.kbAll(), m.kbEsc()},
		}
	case scrSubtitles:
		return screenView{
			title:      strings.TrimSpace(u.SubtitleTitle),
			subtitle:   m.subtitleSubtitle(),
			body:       m.viewSubtitles(),
			notice:     m.flowErr,
			noticeKind: noticeWarn,
			bindings:   []binding{m.kbMove(), m.kbSpace(), m.kbEnter(), m.kbAll(), m.kbEsc()},
		}
	case scrWorkers:
		return m.menuScreen(u.ParallelFmt, fmt.Sprintf(u.WorkersQueuedFmt, len(m.dlEntries)), "")

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

// loadingScreen is the single shared spinner screen for all fetch states.
func (m Model) loadingScreen(title string) screenView {
	return screenView{
		title: title,
		body:  m.renderSpinnerScreen(title),
	}
}

func (m Model) menuScreen(title, subtitle, notice string) screenView {
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

// inputScreen is the single shared text-input screen.
func (m Model) inputScreen(title, subtitle, body, notice string) screenView {
	return screenView{
		title:      strings.TrimSpace(title),
		subtitle:   strings.TrimSpace(subtitle),
		body:       body,
		notice:     notice,
		noticeKind: noticeError,
		bindings:   []binding{m.kbEnter(), m.kbEsc()},
	}
}

// progressMeta renders the shared "pct · bytes · speed" meta line used by
// dependency progress and download slots.
func progressMeta(locale core.Locale, pct float64, doneB, totalB int64, speed string) string {
	if doneB <= 0 && totalB <= 0 && strings.TrimSpace(speed) == "" {
		return sOk.Render(fmt.Sprintf("%.1f%%", pct))
	}
	return sOk.Render(fmt.Sprintf("%.1f%%", pct)) + "  " + fmtStats(locale, doneB, totalB, speed)
}

// counterLine renders the shared "selected/total · queue · elapsed" line.
func (m Model) counterLine(done, failed, total int) string {
	queued := max(0, total-(done+failed))
	return fmt.Sprintf(m.u().QueueFmt, queued) + "  ·  " + formatElapsed(m.dlElapsed)
}

// locationBlock renders the shared downloads-folder section.
func (m Model) locationBlock(title string) string {
	return m.renderSectionBlock(title, renderFileLink(m.api.DownloadsDir()))
}

// ---------- key binding labels ----------
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
	case scrQualityFetch, scrAudioTrackFetch, scrSubsFetch:
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
	if durationText := i18n.FragmentDurationText(m.locale, m.mediaDuration); durationText != "" {
		lines = append(lines, durationText)
	}
	if m.canUseURLStartFragment() {
		lines = append(lines, fmt.Sprintf(u.FragmentFromURLFmt, core.FormatClockTimestamp(m.target.URLStartAt)))
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
		return m.counterLine(m.dlDone, m.dlFailed, m.dlTotal)
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
