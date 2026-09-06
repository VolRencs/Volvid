package core

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

type SearchResult struct {
	Title    string
	URL      string
	Duration int
}
