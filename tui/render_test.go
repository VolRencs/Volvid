package tui

import (
	"fmt"
	"testing"

	app "volvid/internal/app"

	tea "charm.land/bubbletea/v2"
)

func mustModel(t *testing.T, tm tea.Model) Model {
	t.Helper()
	m, ok := tm.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", tm)
	}
	return m
}

func renderAllScreens(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 110, 40
	m.menu.SetItems([]string{"One", "Two", "Three"})
	m.menu.SetCursor(1)
	m.searchResults = []app.SearchResult{{Title: "Result one", Duration: 10}, {Title: "Result two", Duration: 20}}
	m.qualityChoices = app.DefaultQualityChoices()
	m.audioProfiles = app.AudioOutputProfiles(app.LocaleEN)
	m.videoProfiles = app.VideoOutputProfiles(m.qualityChoices[0].Profile(app.LocaleEN), app.LocaleEN)
	m.plInfo = &app.PlaylistInfo{Title: "Test playlist", Entries: []app.PlaylistEntry{
		{Index: 1, Title: "Video one", Duration: 10, URL: "https://youtu.be/1"},
		{Index: 2, Title: "Video two", Duration: 20, URL: "https://youtu.be/2"},
		{Index: 3, Title: "Video three", Duration: 30, URL: "https://youtu.be/3"},
	}}
	m.slots = []slotState{{title: "Downloading something", pct: 50, doneB: 100, totalB: 200, speed: "1.2MiB/s"}}
	m.singleOK = true
	m.dlDone = 1
	m.dlTotal = 0
	m.session = app.Session{Success: 1, Failed: 0, Items: []app.SessionItem{{Label: "MP3 320k", URL: "https://youtu.be/1", OK: true}}}
	m.deps = app.CheckDepsResult{
		YTDLP:  app.DependencyInfo{Name: "yt-dlp", Available: true, Version: "2026.08.01", Source: app.DepManaged, Required: true},
		FFmpeg: app.DependencyInfo{Name: "ffmpeg", Available: true, Version: "7.1", Source: app.DepSystem, Required: true},
	}
	m.depMode = depModeManage
	m.depRefreshing = false

	screens := []screen{
		scrUpdateCheck, scrUpdateReady, scrUpdateDl, scrUpdateDone,
		scrDepDl, scrDepUpdate, scrURL, scrSearchInput, scrSearchResults,
		scrPlaylistAsk, scrPlaylist, scrPlaylistFetch, scrFragmentChoice,
		scrFragmentInput, scrFragmentProbe, scrMode, scrAudio, scrQualityFetch,
		scrQuality, scrVideoOutput, scrWorkers, scrDownload, scrSummary,
	}

	for _, s := range screens {
		m.screen = s
		if s == scrWorkers {
			m.dlEntries = []app.PlaylistEntry{
				{Index: 1, Title: "Video one", URL: "https://youtu.be/1"},
				{Index: 2, Title: "Video two", URL: "https://youtu.be/2"},
			}
		}
		if s == scrUpdateDl || s == scrDepDl {
			m.depProgress = app.FileProgress{Pct: 42.5, DoneB: 500, TotalB: 1000, Speed: "1MiB/s"}
		}
		m = m.syncMenu()
		if s == scrPlaylist {
			m.plCursor = 1
		}
		out := m.View().Content
		if out == "" {
			t.Errorf("screen %d: empty view", s)
			continue
		}
		if len(out) < 20 {
			t.Errorf("screen %d: suspiciously short view: %q", s, out)
		}
	}
}

func TestRenderAllScreens(t *testing.T) {
	renderAllScreens(t)
}

func TestDigitHandling(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 110, 40
	m.screen = scrMode
	m = m.syncMenu()

	// pressing "2" on mode screen should select "Audio" and go to scrAudio
	got := mustModel(t, mustTeaModel(m.handleKey(tea.KeyPressMsg{Text: "2"})))
	if got.screen != scrAudio {
		t.Fatalf("expected scrAudio after digit 2 on mode screen, got %d", got.screen)
	}

	// multi-digit on search results: item 12 should be selectable
	m.screen = scrSearchResults
	results := make([]app.SearchResult, 15)
	for i := range results {
		results[i] = app.SearchResult{Title: fmt.Sprintf("Result %d", i+1), URL: "https://youtu.be/" + fmt.Sprint(i+1)}
	}
	m.searchResults = results
	m = m.syncMenu()
	got = mustModel(t, mustTeaModel(m.handleKey(tea.KeyPressMsg{Text: "1"})))
	got = mustModel(t, mustTeaModel(got.handleKey(tea.KeyPressMsg{Text: "2"})))
	if got.screen != scrFragmentProbe {
		t.Fatalf("expected scrFragmentProbe after typing 12, got %d", got.screen)
	}
}

func TestOpenFolderNotConsumedByURLClick(t *testing.T) {
	m := newTestModel()
	m.screen = scrURL
	m.urlInput.SetValue("https://youtu.be/abc")
	m.urlInput.Focus()
	before := m.urlInput.Value()
	got := mustModel(t, mustTeaModel(m.handleKey(tea.KeyPressMsg{Text: "o"})))
	if got.urlInput.Value() != before+"o" {
		t.Fatalf("expected 'o' typed into URL input, got %q", got.urlInput.Value())
	}
}

func TestVisibleWindow(t *testing.T) {
	i := newInput(inputURL, "placeholder", 10, 100)
	i.value = []rune("abcdefghijklmnop")
	i.offset = 0
	i.cursor = 15
	i.ensureCursorVisible()
	start, end := i.visibleWindow()
	if end-start != 10 {
		t.Fatalf("expected window of width 10, got [%d, %d)", start, end)
	}
	if i.cursor < start || i.cursor >= end {
		t.Fatalf("expected cursor %d inside window [%d, %d)", i.cursor, start, end)
	}
}

func TestCursorVisibleWhenTextFillsField(t *testing.T) {
	i := newInput(inputURL, "placeholder", 10, 100)
	i.SetValue("abcdefghij")
	if i.cursor != len(i.value) {
		t.Fatalf("expected cursor at end after SetValue, got %d", i.cursor)
	}
	i.ensureCursorVisible()
	start, end := i.visibleWindow()
	if start > i.cursor || i.cursor > end {
		t.Fatalf("cursor %d outside window [%d, %d)", i.cursor, start, end)
	}
	if i.cursor == end && end-start >= i.width {
		t.Fatalf("tail cursor unreachable: window [%d, %d) already full", start, end)
	}

	i.cursor--
	i.ensureCursorVisible()
	start, end = i.visibleWindow()
	if i.cursor < start || i.cursor >= end {
		t.Fatalf("cursor %d not inline-visible in window [%d, %d)", i.cursor, start, end)
	}
}

func TestCancelResetDownloadState(t *testing.T) {
	m := newTestModel()
	m.dlCancelled = true
	m.resetDownloadState()
	if m.dlCancelled {
		t.Fatal("expected dlCancelled to be reset by resetDownloadState")
	}
}

func mustTeaModel(tm tea.Model, _ tea.Cmd) tea.Model { return tm }
