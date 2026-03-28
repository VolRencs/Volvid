package bot

import app "YouTubeBuild/internal/app"

func (s *Session) snapshot() SessionSnapshot {
	if s == nil {
		return SessionSnapshot{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionSnapshot{
		UserID:          s.UserID,
		State:           s.State,
		URL:             s.URL,
		Target:          s.Target,
		WorkDir:         s.WorkDir,
		SearchQuery:     s.SearchQuery,
		SearchResults:   append([]app.SearchResult(nil), s.SearchResults...),
		PlInfo:          clonePlaylistInfo(s.PlInfo),
		SelectedIndices: cloneSelection(s.SelectedIndices),
		PlaylistPage:    s.PlaylistPage,
		QualityChoices:  append([]app.QualityChoice(nil), s.QualityChoices...),
		Mode:            s.Mode,
		Profile:         s.Profile,
		MediaDuration:   s.MediaDuration,
		Fragment:        cloneDownloadFragment(s.Fragment),
		ForceSingle:     s.ForceSingle,
		StatusMsgID:     s.StatusMsgID,
	}
}

func cloneDownloadFragment(fragment *app.DownloadFragment) *app.DownloadFragment {
	if fragment == nil {
		return nil
	}
	cloned := *fragment
	return &cloned
}

func clonePlaylistInfo(info *app.PlaylistInfo) *app.PlaylistInfo {
	if info == nil {
		return nil
	}
	cloned := &app.PlaylistInfo{
		Title:   info.Title,
		Entries: append([]app.PlaylistEntry(nil), info.Entries...),
	}
	return cloned
}

func cloneSelection(src map[int]bool) map[int]bool {
	if len(src) == 0 {
		return nil
	}
	out := make(map[int]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
