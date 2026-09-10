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
	m.subList.selected = map[string]bool{}
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
	m.subList.selected = map[string]bool{"en": true}
	m.subList.cursor = 0
	got := confirmViaEnter(m)
	if got.profile.SubMode != core.SubOff || len(got.profile.SubLangs) != 0 {
		t.Fatalf("expected subs off, got %+v", got.profile)
	}
}

func TestSubtitleMultiSelect(t *testing.T) {
	m := subtitleTestModel()
	m.subList.cursor = 1
	model, _ := m.handleSubtitlesKey(subKey("space")) // en
	m = model.(Model)
	m.subList.cursor = 2
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
	if len(m.subList.selected) != 2 {
		t.Fatalf("expected all selected, got %v", m.subList.selected)
	}
	model, _ = m.handleSubtitlesKey(subKey("a"))
	m = model.(Model)
	if len(m.subList.selected) != 0 {
		t.Fatalf("expected none selected, got %v", m.subList.selected)
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
	if m.subList.cursor != len(m.subTracks) {
		t.Fatalf("expected cursor at end, got %d", m.subList.cursor)
	}
	body := m.viewSubtitles()
	if body == "" {
		t.Fatal("expected non-empty subtitles view")
	}
}

func audioTrackTestModel() Model {
	stub := newStubAPI()
	m := newStubModel(stub)
	m.locale = core.LocaleEN
	m.audioTracks = []core.AudioTrack{{Lang: "en"}, {Lang: "ru"}}
	m.audioOffered = true
	m.audioList.selected = map[string]bool{}
	m.screen = scrAudioTrack
	m = m.syncMenu()
	return m
}

func confirmAudioViaEnter(m Model) Model {
	model, _ := m.handleAudioTrackKey(subKey("enter"))
	return model.(Model)
}

func TestAudioTrackRowZeroSkipsImmediately(t *testing.T) {
	m := audioTrackTestModel()
	m.target = core.ParsedTarget{Kind: core.TargetVideo, CanonicalURL: "https://www.youtube.com/watch?v=x"}
	m.audioList.selected = map[string]bool{"en": true}
	m.audioList.cursor = 0
	m.subTracks = []core.SubtitleTrack{{Lang: "en"}}
	m.subsOffered = true
	got := confirmAudioViaEnter(m)
	if len(got.profile.AudioLangs) != 0 {
		t.Fatalf("expected no override, got %v", got.profile.AudioLangs)
	}
	if got.screen != scrSubtitles {
		t.Fatalf("expected subs picker (resolved in batch), got %v", got.screen)
	}
}

func TestAudioTrackMultiSelect(t *testing.T) {
	m := audioTrackTestModel()
	m.audioList.cursor = 1
	model, _ := m.handleAudioTrackKey(subKey("space")) // en
	m = model.(Model)
	m.audioList.cursor = 2
	model, _ = m.handleAudioTrackKey(subKey("space")) // ru
	m = model.(Model)

	got := confirmAudioViaEnter(m)
	if len(got.profile.AudioLangs) != 2 || got.profile.AudioLangs[0] != "en" || got.profile.AudioLangs[1] != "ru" {
		t.Fatalf("expected [en ru] in order, got %v", got.profile.AudioLangs)
	}
}

func TestAudioTracksLoadedSkipsWhenSingle(t *testing.T) {
	stub := newStubAPI()
	m := newStubModel(stub)
	m.locale = core.LocaleEN
	model, _ := m.handleTracksLoaded(msgTracksLoaded{
		audioTracks: []core.AudioTrack{{Lang: "en"}},
		subTracks:   []core.SubtitleTrack{{Lang: "en"}},
		gen:         m.opGen,
	})
	got := model.(Model)
	if got.audioOffered {
		t.Fatal("expected no picker for a single track")
	}
	if !got.subsOffered || got.screen != scrSubtitles {
		t.Fatalf("expected subs picker, got screen %v", got.screen)
	}
}

func TestTracksLoadedSkipsBothWhenEmpty(t *testing.T) {
	stub := newStubAPI()
	m := newStubModel(stub)
	m.locale = core.LocaleEN
	m.target = core.ParsedTarget{Kind: core.TargetVideo, CanonicalURL: "https://www.youtube.com/watch?v=x"}
	model, _ := m.handleTracksLoaded(msgTracksLoaded{gen: m.opGen})
	got := model.(Model)
	if got.audioOffered || got.subsOffered {
		t.Fatal("expected no pickers without tracks")
	}
}

func TestAudioTrackToggleAll(t *testing.T) {
	m := audioTrackTestModel()
	model, _ := m.handleAudioTrackKey(subKey("a"))
	m = model.(Model)
	if len(m.audioList.selected) != 2 {
		t.Fatalf("expected all selected, got %v", m.audioList.selected)
	}
}
