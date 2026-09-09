package core

import "errors"

// ErrDownloadsDirLocked is returned when the download location is fixed
// via VOLVID_DOWNLOADS_DIR and the user tries to change it.
var ErrDownloadsDirLocked = errors.New("download location is fixed by VOLVID_DOWNLOADS_DIR")

// UpdateInfo describes a newer release found on GitHub.
type UpdateInfo struct {
	Latest string
	DlURL  string
}

type PlaylistEntry struct {
	Index    int
	Title    string
	URL      string
	Duration int
}

type PlaylistInfo struct {
	Title   string
	Entries []PlaylistEntry
}

// SearchResult is a single YouTube search hit.
// It mirrors PlaylistEntry without the playlist index.
type SearchResult struct {
	Title    string
	URL      string
	Duration int
}

// ToPlaylistEntry converts a search hit into a playlist entry.
func (s SearchResult) ToPlaylistEntry(index int) PlaylistEntry {
	return PlaylistEntry{Index: index, Title: s.Title, URL: s.URL, Duration: s.Duration}
}

type SessionItem struct {
	Label string
	URL   string
	OK    bool
}

// Session accumulates per-run download history.
// Must only be used from the UI goroutine (no locking).
type Session struct {
	Success int
	Failed  int
	Items   []SessionItem
}

func (s *Session) Record(label, url string, ok bool) {
	if ok {
		s.Success++
	} else {
		s.Failed++
	}
	s.Items = append(s.Items, SessionItem{Label: label, URL: url, OK: ok})
}
