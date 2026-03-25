package app

import "strings"

func requestUsesPlaylist(req DownloadRequest) bool {
	req = NormalizeDownloadRequest(req)
	return req.PlaylistInfo != nil && !req.ForceSingle && len(req.Entries) > 0
}

func requestEntries(req DownloadRequest) []PlaylistEntry {
	if !requestUsesPlaylist(req) {
		return nil
	}
	return append([]PlaylistEntry(nil), req.Entries...)
}

func requestSourceURLs(req DownloadRequest) []string {
	req = NormalizeDownloadRequest(req)

	if entries := requestEntries(req); len(entries) > 0 {
		urls := make([]string, 0, len(entries))
		for _, entry := range entries {
			if strings.TrimSpace(entry.URL) == "" {
				continue
			}
			urls = append(urls, entry.URL)
		}
		return urls
	}

	return []string{req.Target.DownloadURL(req.ForceSingle)}
}
