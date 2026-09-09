package tui

import (
	"testing"
	"volvid/internal/core"

	tea "charm.land/bubbletea/v2"
)

func subKey(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		return tea.KeyPressMsg{Text: name}
	}
}

func subtitleTestModel() Model {
	stub := newStubAPI()
	m := newStubModel(stub)
	m.locale = core.LocaleEN
	m.subTracks = []core.SubtitleTrack{{Lang: "en"}, {Lang: "ru", Auto: true}}
	m.subsOffered = true
	m.subSelected = map[string]bool{}
	m.screen = scrSubtitles
	m = m.syncMenu()
	return m
}

func confirmViaEnter(m Model) Model {
	model, _ := m.handleSubtitlesKey(subKey("enter"))
	return model.(Model)
}

func TestSubtitleConfirmEmptyMeansOff(t *testing.T) {
	got := confirmViaEnter(subtitleTestModel())
	if got.profile.SubMode != core.SubOff || len(got.profile.SubLangs) != 0 {
		t.Fatalf("expected subs off, got %+v", got.profile)
	}
}

func TestSubtitleRowZeroSkipsImmediately(t *testing.T) {
	m := subtitleTestModel()
	// Even with languages checked, cursor on "no subtitles" means off.
	m.subSelected = map[string]bool{"en": true}
	m.subCursor = 0
	got := confirmViaEnter(m)
	if got.profile.SubMode != core.SubOff || len(got.profile.SubLangs) != 0 {
		t.Fatalf("expected subs off, got %+v", got.profile)
	}
}

func TestSubtitleMultiSelect(t *testing.T) {
	m := subtitleTestModel()
	m.subCursor = 1
	model, _ := m.handleSubtitlesKey(subKey("space")) // en
	m = model.(Model)
	m.subCursor = 2
	model, _ = m.handleSubtitlesKey(subKey("space")) // ru
	m = model.(Model)

	got := confirmViaEnter(m)
	if got.profile.SubMode != core.SubEmbed {
		t.Fatalf("expected embed mode, got %+v", got.profile)
	}
	if len(got.profile.SubLangs) != 2 || got.profile.SubLangs[0] != "en" || got.profile.SubLangs[1] != "ru" {
		t.Fatalf("expected [en ru] in order, got %v", got.profile.SubLangs)
	}
}

func TestSubtitleToggleAll(t *testing.T) {
	m := subtitleTestModel()
	model, _ := m.handleSubtitlesKey(subKey("a"))
	m = model.(Model)
	if len(m.subSelected) != 2 {
		t.Fatalf("expected all selected, got %v", m.subSelected)
	}
	model, _ = m.handleSubtitlesKey(subKey("a"))
	m = model.(Model)
	if len(m.subSelected) != 0 {
		t.Fatalf("expected none selected, got %v", m.subSelected)
	}
}

func TestSubtitleCursorStaysInViewport(t *testing.T) {
	m := subtitleTestModel()
	for i := 0; i < 10; i++ {
		m.subTracks = append(m.subTracks, core.SubtitleTrack{Lang: "l" + string(rune('a'+i))})
	}
	for i := 0; i < 20; i++ {
		model, _ := m.handleSubtitlesKey(subKey("down"))
		m = model.(Model)
	}
	if m.subCursor != len(m.subTracks) {
		t.Fatalf("expected cursor at end, got %d", m.subCursor)
	}
	body := m.viewSubtitles()
	if body == "" {
		t.Fatal("expected non-empty subtitles view")
	}
}
