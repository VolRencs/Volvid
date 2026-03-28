package app

import "strings"

func requestUsesPlaylist(req DownloadRequest) bool {
	return req.PlaylistInfo != nil && !req.ForceSingle && len(req.Entries) > 0
}

func requestEntries(req DownloadRequest) []PlaylistEntry {
	if !requestUsesPlaylist(req) {
		return nil
	}
	return append([]PlaylistEntry(nil), req.Entries...)
}

func requestSourceURLs(req DownloadRequest) []string {
	if entries := requestEntries(req); len(entries) > 0 {
		return entryURLs(entries)
	}

	return []string{req.Target.DownloadURL(req.ForceSingle)}
}

func entryURLs(entries []PlaylistEntry) []string {
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		url := strings.TrimSpace(entry.URL)
		if url == "" {
			continue
		}
		urls = append(urls, url)
	}
	return urls
}
