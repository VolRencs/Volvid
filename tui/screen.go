package tui

// Screen state-machine: every value is a distinct UI state with
// rendering + input + async-transition rules. Adding a screen means
// adding one case here, one branch in screenView (view.go), one item
// source in menuItems (menu.go) and one transition in flow.go —
// all screen knowledge is grouped via props() instead of scattered
// switch statements.
type screen int

const (
	scrUpdateCheck screen = iota
	scrUpdateReady
	scrUpdateDl
	scrUpdateDone
	scrDepDl
	scrDepUpdate
	scrURL
	scrSearchInput
	scrSearchFetch
	scrSearchResults
	scrPlaylistAsk
	scrPlaylistFetch
	scrPlaylist
	scrFragmentProbe
	scrFragmentChoice
	scrFragmentInput
	scrMode
	scrAudio
	scrQualityFetch
	scrQuality
	scrVideoOutput
	scrWorkers
	scrDownload
	scrSummary
)

type screenProps struct {
	spinner  bool
	menu     bool
	busy     bool
	updating bool
}

func (s screen) props() screenProps {
	switch s {
	case scrUpdateCheck:
		return screenProps{spinner: true, busy: true, updating: true}
	case scrUpdateReady:
		return screenProps{menu: true, updating: true}
	case scrUpdateDl:
		return screenProps{busy: true, updating: true}
	case scrUpdateDone:
		return screenProps{updating: true}
	case scrDepDl:
		return screenProps{busy: true}
	case scrDepUpdate:
		return screenProps{menu: true, busy: true}
	case scrSearchFetch, scrPlaylistFetch, scrFragmentProbe, scrQualityFetch:
		return screenProps{spinner: true, busy: true}
	case scrPlaylistAsk, scrSearchResults, scrFragmentChoice, scrMode, scrAudio, scrQuality, scrVideoOutput, scrWorkers, scrSummary:
		return screenProps{menu: true}
	case scrDownload:
		return screenProps{busy: true}
	default:
		return screenProps{}
	}
}

// Screen predicates (formerly scattered across state_menu.go).
func (m Model) uiBusy() bool { return m.screen.props().busy }

func (m Model) isAppUpdateScreen() bool { return m.screen.props().updating }

func (m Model) canOpenDependencyScreen() bool {
	return !m.uiBusy() && !m.isAppUpdateScreen()
}

func (m Model) canOpenDownloadsFolder() bool {
	if m.screen == scrURL {
		return true
	}
	return m.screen == scrSummary && (m.singleOK || m.dlDone > 0)
}

func (m Model) canPickDownloadsFolder() bool { return m.screen == scrURL }

func (m Model) isMenuScreen() bool { return m.screen.props().menu }

func (m Model) spinnerVisible() bool { return m.screen.props().spinner }
