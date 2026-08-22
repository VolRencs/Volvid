package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

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

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m Model) View() tea.View {
	content := m.buildScreen(m.renderCard(m.screenView()))
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "VolRen Downloader · v" + app.Version
	v.Cursor = nil
	return v
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
			title:      m.depScreenTitle(),
			subtitle:   m.depScreenSubtitle(),
			body:       m.renderDepsUpdateScreen(),
			notice:     notice,
			noticeKind: kind,
			bindings:   m.depBindings(),
		}

	case scrURL:
		return screenView{
			title:      strings.TrimSpace(u.PasteURL),
			subtitle:   strings.TrimSpace(u.URLHints),
			body:       m.renderURLScreenBody(),
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
		title: "VolRen Downloader",
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

func (m Model) renderURLScreenBody() string {
	parts := []string{
		m.renderHomeInput(),
		m.renderDownloadsLocation(),
	}
	if section := m.renderHomeSession(); section != "" {
		parts = append(parts, section)
	}
	return strings.Join(compactSections(parts...), m.sectionGap())
}

func (m Model) renderHomeInput() string {
	title := sSectionTitle.Render(m.u().HomeInputTitle)
	return title + "\n" + renderInputField(m.urlInput)
}

func (m Model) renderDownloadsLocation() string {
	pathWidth := max(18, m.cardBodyWidth()-10)
	body := renderFileLink(trunc(m.env.DownloadsDir(), pathWidth))
	return m.renderSectionBlock(m.u().HomeOutputTitle, body)
}

func (m Model) renderHomeSession() string {
	if m.compactHomeLayout() && len(m.session.Items) == 0 {
		return ""
	}

	stats := renderBadge(m.u().HomeStatSuccess, strconv.Itoa(m.session.Success)) + "  " +
		renderBadge(m.u().HomeStatFailed, strconv.Itoa(m.session.Failed))
	if len(m.session.Items) == 0 {
		return m.renderSectionBlock(m.u().HomeSessionTitle, stats+"\n"+sMeta.Render(m.u().HomeSessionEmpty))
	}

	items := m.session.Items
	if len(items) > 3 {
		items = items[len(items)-3:]
	}

	width := max(18, m.cardBodyWidth()/2)
	rows := make([]string, 0, len(items)+1)
	rows = append(rows, stats)
	for _, item := range items {
		icon := sessionIcon(item.OK)
		rows = append(rows, icon+"  "+sValue.Render(trunc(item.Label, width))+"\n"+sMeta.Render(trunc(item.URL, width+14)))
	}
	return m.renderSectionBlock(m.u().HomeSessionTitle, strings.Join(rows, "\n\n"))
}

func sessionIcon(ok bool) string {
	if ok {
		return sOk.Render("●")
	}
	return sErr.Render("●")
}

func compactSections(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
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
	left := sBold.Render("VolRen") + " " + sSubtitle.Render("Downloader") + sDim.Render("  v"+app.Version)
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
	if m.width > 0 && m.width < 72 {
		if m.canOpenDependencyScreen() {
			return renderActionBadge("Ctrl+U", m.u().HelpDeps)
		}
		return ""
	}

	parts := []string{
		renderBadge("yt-dlp", versionBadgeValue(m.deps.YTDLP.Version)),
		renderBadge("ffmpeg", versionBadgeValue(m.deps.FFmpeg.Version)),
	}
	if m.width > 0 && m.width < 90 {
		return strings.Join(parts, " ")
	}
	if m.canOpenDependencyScreen() {
		parts = append(parts, renderActionBadge("Ctrl+U", m.u().HelpDeps))
	}
	return strings.Join(parts, " ")
}

func (m Model) renderLocaleFooter() string {
	hint := renderActionBadge("Tab", strings.ToUpper(m.locale.String()))
	if m.width == 0 {
		return hint
	}
	return lipgloss.Place(m.width, 1, lipgloss.Right, lipgloss.Top, hint)
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

func (m Model) renderInputWithHint(field inputField, hint string) string {
	parts := []string{renderInputField(field)}
	if hint = strings.TrimSpace(hint); hint != "" {
		parts = append(parts, sInputHint.Render(hint))
	}
	return strings.Join(parts, "\n")
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
func (m Model) spinnerView() string { return spinnerFrames[m.spinnerFrame%len(spinnerFrames)] }

func (m Model) renderFooterHelp(bindings ...binding) string {
	parts := make([]string, 0, len(bindings))
	for _, item := range bindings {
		parts = append(parts, sHelpKey.Render(item.key)+" "+sHelpText.Render(item.help))
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

func (m Model) depScreenTitle() string {
	return m.u().DepTitle
}

func (m Model) depScreenSubtitle() string {
	if m.depRefreshing {
		return m.u().DepsRefreshing
	}
	return m.u().DepSubtitle
}

func (m Model) renderUpdateDone() string {
	if m.env.IsWindows {
		return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedWin))
	}
	return m.renderSectionBlock("", sMeta.Render(m.u().UpdateAppliedUnix))
}

func (m Model) renderDepsUpdateScreen() string {
	rows := make([]depStatusRow, 0, len(m.deps.Dependencies())+2)
	for _, dep := range m.deps.Dependencies() {
		rows = append(rows, depStatusRow{Label: dep.Name, Value: m.depLineValue(dep)})
	}
	rows = append(rows,
		depStatusRow{Label: "cookies", Value: m.depAccessValue(m.deps.Cookies.Status, m.cookiesAccessDetail())},
		depStatusRow{Label: "js", Value: m.depAccessValue(m.deps.Runtime.Status, m.runtimeAccessDetail())},
	)

	parts := []string{m.renderSectionBlock("", m.renderDepStatusRows(rows))}
	if systemCount := m.systemDepsCount(); systemCount > 0 {
		parts = append(parts, m.renderSectionBlock("", sMeta.Render(m.u().DepSystemNote)))
	}
	if len(m.menu.items) > 0 {
		parts = append(parts, m.menu.View(m.menuWidth()))
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) systemDepsCount() int {
	count := 0
	for _, dep := range m.deps.Dependencies() {
		if dep.Source == app.DepSystem {
			count++
		}
	}
	return count
}

type depStatusRow struct {
	Label string
	Value string
}

func (m Model) renderDepStatusRows(rows []depStatusRow) string {
	labelWidth := 0
	for _, row := range rows {
		labelWidth = max(labelWidth, lipgloss.Width(strings.TrimSpace(row.Label)))
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		label := fmt.Sprintf("%-*s", labelWidth, strings.TrimSpace(row.Label))
		lines = append(lines, sTableLabel.Render(label)+sMeta.Render("  ·  ")+sTableMeta.Render(row.Value))
	}
	return strings.Join(lines, "\n")
}

func (m Model) depLineValue(dep app.DependencyInfo) string {
	role := m.depRoleText(dep)
	if !dep.Available {
		return sErr.Render(m.depText("missing")) + sDim.Render("  ["+role+"]")
	}

	meta := []string{m.depSourceText(dep.Source), role}
	version := dep.Version
	checking := strings.TrimSpace(version) == "" && m.depRefreshing
	if checking {
		version = m.depText("checking")
	}
	if strings.TrimSpace(version) == "" {
		version = m.depText("available")
	}
	if checking {
		return sDim.Render(version) + sDim.Render("  ["+strings.Join(meta, ", ")+"]")
	}
	return sOk.Render(version) + sDim.Render("  ["+strings.Join(meta, ", ")+"]")
}

func (m Model) depAccessValue(status, detail string) string {
	status = strings.TrimSpace(status)
	detail = strings.TrimSpace(detail)
	if m.depRefreshing && status == "" {
		return sDim.Render(m.depText("checking"))
	}
	switch status {
	case "", "browser not found", "not found":
		return sDim.Render(m.depText("not_active"))
	case "active":
		if detail == "" {
			return sOk.Render(m.depText("active"))
		}
		return sOk.Render(detail) + sDim.Render("  ["+status+"]")
	default:
		if detail == "" {
			return sWarn.Render(status)
		}
		return sWarn.Render(detail) + sDim.Render("  ["+status+"]")
	}
}

func (m Model) depText(kind string) string {
	u := m.u()
	switch kind {
	case "active":
		return u.DepStatusActive
	case "missing":
		return u.DepStatusMissing
	case "not_active":
		return u.DepStatusNotActive
	case "available":
		return u.DepStatusAvailable
	case "checking":
		return u.DepStatusChecking
	}
	return kind
}

func (m Model) cookiesAccessDetail() string {
	browser := strings.TrimSpace(m.deps.Cookies.Browser)
	if browser == "" {
		return ""
	}
	if profile := strings.TrimSpace(m.deps.Cookies.ProfileName); profile != "" {
		return browser + ":" + profile
	}
	return browser
}

func (m Model) runtimeAccessDetail() string {
	return strings.TrimSpace(m.deps.Runtime.Name)
}

func (m Model) depRoleText(dep app.DependencyInfo) string {
	if dep.Required {
		return m.u().DepRoleRequired
	}
	return m.u().DepRoleOptional
}

func (m Model) depSourceText(source app.DependencySource) string {
	switch source {
	case app.DepManaged:
		return m.u().DepSourceBundled
	case app.DepSystem:
		return m.u().DepSourceSystem
	default:
		return string(source)
	}
}

func (m Model) viewDependencyProgress() string {
	lines := []string{renderProgressBar(m.progressBarWidth(), m.depProgress.Pct)}
	var meta strings.Builder
	meta.WriteString(sOk.Render(fmt.Sprintf("%.1f%%", m.depProgress.Pct)))
	if m.depProgress.DoneB > 0 {
		meta.WriteString("  " + sValue.Render(app.FmtBytesFor(m.depProgress.DoneB, m.locale)))
		if m.depProgress.TotalB > 0 {
			meta.WriteString(sMeta.Render(" / " + app.FmtBytesFor(m.depProgress.TotalB, m.locale)))
		}
		if m.depProgress.Speed != "" {
			meta.WriteString("  " + sTitle.Render(m.depProgress.Speed))
		}
	}
	lines = append(lines, meta.String())
	return m.renderSectionBlock("", strings.Join(lines, "\n"))
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
	staticWidth := lipgloss.Width(renderMenuLead(false)) + lipgloss.Width("●") + indexWidth + 14
	titleWidth := max(1, min(m.playlistTitleWidth(), rowWidth-staticWidth))

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		entry := entries[i]
		current := i == m.plCursor
		lead := renderMenuLead(current)

		check := sMeta.Render("○")
		if m.plSelected[entry.Index] {
			check = sOk.Render("●")
		}

		number := sMenuIndex.Width(indexWidth).Render(fmt.Sprintf("%*d", indexWidth, entry.Index))
		title := sMenuText.Render(sPlTitle.Width(titleWidth).Render(trunc(entry.Title, titleWidth)))
		if current {
			number = sMenuIndexAct.Width(indexWidth).Render(fmt.Sprintf("%*d", indexWidth, entry.Index))
			title = sMenuTextAct.Render(sPlTitle.Width(titleWidth).Render(trunc(entry.Title, titleWidth)))
		}
		duration := sTableMeta.Render(fmt.Sprintf("%8s", app.FmtDuration(entry.Duration)))
		row := lead + check + "  " + number + "  " + title + "  " + duration
		style := sMenuRow
		if current {
			style = sMenuActive
		}
		lines = append(lines, style.Render(row))
	}
	return strings.Join(lines, "\n")
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

func (m Model) viewDownload() string {
	u := m.u()
	var parts []string
	barWidth := m.progressBarWidth()

	if m.dlTotal > 0 {
		done := m.dlDone + m.dlFailed
		pct := float64(done) / float64(m.dlTotal) * 100
		statsLine := sOk.Render(fmt.Sprintf("%.1f%%", pct)) + "  " +
			sOk.Render(fmt.Sprintf("● %d", m.dlDone)) + "  " +
			sErr.Render(fmt.Sprintf("● %d", m.dlFailed)) + "  " +
			sMeta.Render(fmt.Sprintf(u.QueueFmt, m.dlTotal-done)) + "  " +
			sMeta.Render(formatElapsed(m.dlElapsed))
		parts = append(parts, m.renderSectionBlock("", strings.Join([]string{
			renderProgressBar(barWidth, pct),
			statsLine,
		}, "\n")))
		for i, slot := range m.slots {
			parts = append(parts, m.renderSectionBlock("", m.viewSlot(i, slot, true)))
		}
		return strings.Join(parts, "\n\n")
	}

	if len(m.slots) > 0 {
		return m.renderSectionBlock("", m.viewSlot(0, m.slots[0], false))
	}
	return ""
}

func (m Model) viewSlot(index int, slot slotState, withBadge bool) string {
	u := m.u()

	indicator := sDim.Render("○")
	switch {
	case slot.done:
		indicator = sOk.Render("●")
	case slot.failed:
		indicator = sErr.Render("●")
	case slot.title == "" && !slot.proc:
		indicator = sTitle.Render(m.spinnerView())
	}

	prefix := indicator + "  "
	if withBadge {
		prefix += sLabel.Render(fmt.Sprintf("#%d", index+1)) + "  "
	}

	if slot.title == "" && !slot.done && !slot.failed && !slot.proc && slot.pct <= 0 && slot.doneB <= 0 {
		return prefix + sMeta.Render(u.Waiting)
	}

	titleWidth := max(1, m.cardBodyWidth()-lipgloss.Width(prefix))
	title := trunc(slot.title, titleWidth)
	if strings.TrimSpace(title) == "" {
		title = u.Downloading
	}

	lines := []string{prefix + sValue.Render(sSlotTitle.Width(titleWidth).Render(title))}
	switch {
	case slot.done:
		lines = append(lines, renderProgressBar(m.progressBarWidth(), 100)+"  "+sOk.Render("100%"))
	case slot.failed:
		message := u.ErrSlot
		if strings.TrimSpace(slot.label) != "" {
			message = slot.label
		}
		lines = append(lines, sErr.Render(message))
	case slot.proc:
		lines = append(lines, sWarn.Render(slot.label))
	default:
		lines = append(lines, renderProgressBar(m.progressBarWidth(), slot.pct)+"  "+sOk.Render(fmt.Sprintf("%.1f%%", slot.pct))+"  "+fmtStats(m.locale, slot.doneB, slot.totalB, slot.speed))
	}
	return strings.Join(lines, "\n")
}

func (m Model) summaryTitle() string {
	if m.dlTotal > 0 {
		switch {
		case m.dlDone == 0 && m.dlFailed > 0:
			return m.u().SummaryFail
		case m.dlFailed > 0:
			return m.u().SummaryPartial
		default:
			return m.u().SummaryOK
		}
	}
	if !m.singleOK {
		return m.u().SummaryFail
	}
	return m.u().SummaryOK
}

func (m Model) summarySubtitle() string {
	if m.dlTotal > 0 {
		return fmt.Sprintf("%s %d  ·  %s %d  ·  %s",
			sOk.Render("●"), m.dlDone,
			sErr.Render("●"), m.dlFailed,
			formatElapsed(m.dlElapsed))
	}
	return formatElapsed(m.dlElapsed)
}

func (m Model) viewSummary() string {
	var parts []string

	if m.singleOK || m.dlDone > 0 {
		parts = append(parts, m.renderSectionBlock(m.u().SummaryLocation, renderFileLink(m.env.DownloadsDir())))
	}
	if m.dlTotal > 0 {
		playlistLine := sValue.Render(m.u().SummaryPlaylistTitle) + "  " +
			sOk.Render(fmt.Sprintf("● %d", m.dlDone)) + "  " +
			sErr.Render(fmt.Sprintf("● %d", m.dlFailed))
		parts = append(parts, m.renderSectionBlock("", playlistLine))
	}

	if len(m.menu.items) > 0 {
		parts = append(parts, m.menu.View(m.menuWidth()))
	}

	if len(m.session.Items) > 0 {
		rows := make([]string, 0, len(m.session.Items))
		labelWidth := fitWidth(m.cardBodyWidth()/3, 24, 8)
		urlWidth := max(1, m.cardBodyWidth()-labelWidth-10)
		for _, item := range m.session.Items {
			icon := sessionIcon(item.OK)
			rows = append(rows, fmt.Sprintf("%s %-*s  %s", icon, labelWidth, trunc(item.Label, labelWidth), sMeta.Render(trunc(item.URL, urlWidth))))
		}
		parts = append(parts, m.renderSectionBlock(m.u().SessionHist, strings.Join(rows, "\n")))
	}

	return strings.Join(parts, "\n\n")
}

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

func renderActionBadge(key, label string) string {
	return sBadge.Render(sBadgeHotkey.Render(key) + " " + sMeta.Render(label))
}

func renderFileLink(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return sLink.Render(path)
}

func openDownloadsDirCmd(path string) tea.Cmd {
	return func() tea.Msg {
		return msgOpenDownloadsDirDone{err: app.OpenInFileManager(path)}
	}
}

func pickDownloadsDirCmd(env *app.Env, path string, locale app.Locale) tea.Cmd {
	return func() tea.Msg {
		dir, err := app.PickDownloadsDir(env, path, locale)
		return msgPickDownloadsDirDone{path: dir, err: err}
	}
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
		return sNoticeTag.Foreground(cSuccess).Render(u.NoticeSuccess) + " "
	case noticeWarn:
		return sNoticeTag.Foreground(cWarn).Render(u.NoticeWarn) + " "
	case noticeError:
		return sNoticeTag.Foreground(cError).Render(u.NoticeError) + " "
	default:
		return ""
	}
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

func (m Model) compactHomeLayout() bool {
	return (m.height > 0 && m.height < 31) || (m.width > 0 && m.width < 78)
}

func (m Model) sectionGap() string {
	if m.height > 0 && m.height < 31 {
		return "\n"
	}
	return "\n\n"
}

func (m Model) sectionBodyWidth() int {
	return max(1, m.cardBodyWidth()-4)
}

func (m Model) cardStyle() lipgloss.Style {
	py, px := m.cardPadding()
	return sCard.Padding(py, px)
}

func sep(width int) string {
	return sRule.Render(strings.Repeat("─", width))
}

func trunc(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	if lipgloss.Width(value) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}

	var b strings.Builder
	width := 0
	for _, r := range value {
		rw := lipgloss.Width(string(r))
		if width+rw > limit-1 {
			break
		}
		b.WriteRune(r)
		width += rw
	}

	out := strings.TrimRight(b.String(), " ")
	if out == "" {
		return "…"
	}
	return out + "…"
}

func fmtStats(l app.Locale, done, total int64, speed string) string {
	switch {
	case total > 0:
		return sValue.Render(app.FmtBytesFor(done, l)) + sDim.Render("/"+app.FmtBytesFor(total, l)) + speedSuffix(speed)
	case done > 0:
		return sValue.Render(app.FmtBytesFor(done, l)) + speedSuffix(speed)
	default:
		return sDim.Render("…")
	}
}

func speedSuffix(speed string) string {
	speed = strings.TrimSpace(speed)
	if speed == "" {
		return ""
	}
	return "  " + sTitle.Render(speed)
}

func joinFittedParts(width int, parts []string, sep string) string {
	parts = compactSections(parts...)
	if len(parts) == 0 {
		return ""
	}
	if width <= 0 {
		return strings.Join(parts, sep)
	}

	lines := make([]string, 0, len(parts))
	current := parts[0]
	for _, part := range parts[1:] {
		candidate := current + sep + part
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = part
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
