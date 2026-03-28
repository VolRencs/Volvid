package bot

import app "YouTubeBuild/internal/app"

func (b *Bot) applyPlaylistSelection(sess *Session, selected map[int]bool) {
	sess.mutate(func(s *Session) {
		s.SelectedIndices = cloneSelection(selected)
	})
}

func (b *Bot) setPlaylistSelection(sess *Session, selected map[int]bool) bool {
	snap := sess.snapshot()
	if !b.validatePlaylistSelectionCount(snap.UserID, len(selected)) {
		return false
	}
	b.applyPlaylistSelection(sess, selected)
	return true
}

func (b *Bot) selectPlaylistIndices(sess *Session, indices []int) bool {
	return b.setPlaylistSelection(sess, selectionMapFromIndices(indices))
}

func (b *Bot) selectAllPlaylistEntries(sess *Session) bool {
	snap := sess.snapshot()
	if snap.PlInfo == nil {
		return false
	}
	return b.setPlaylistSelection(sess, selectionMapForAll(snap.PlInfo.Entries))
}

func (b *Bot) togglePlaylistSelection(sess *Session, idx int) (map[int]bool, bool, bool) {
	snap := sess.snapshot()
	selected := cloneSelection(snap.SelectedIndices)
	if selected == nil {
		selected = make(map[int]bool)
	}
	if selected[idx] {
		delete(selected, idx)
		return selected, true, false
	}
	if idx < 1 || idx > len(snap.PlInfo.Entries) {
		return selected, false, false
	}
	if !b.validatePlaylistSelectionCount(snap.UserID, len(selected)+1) {
		return selected, false, true
	}
	selected[idx] = true
	return selected, true, false
}

func (b *Bot) playlistSelectionEntries(sess *Session) []app.PlaylistEntry {
	snap := sess.snapshot()
	if snap.PlInfo == nil || snap.ForceSingle || len(snap.SelectedIndices) == 0 {
		return nil
	}
	return playlistEntriesFromSelection(snap.PlInfo, snap.SelectedIndices)
}

func (b *Bot) selectedPlaylistCount(sess *Session) int {
	return len(sess.snapshot().SelectedIndices)
}

func (b *Bot) playlistSelectionURLs(sess *Session) []string {
	entries := b.playlistSelectionEntries(sess)
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}
	return urls
}

func selectionMapFromIndices(indices []int) map[int]bool {
	if len(indices) == 0 {
		return nil
	}
	selected := make(map[int]bool, len(indices))
	for _, idx := range indices {
		selected[idx] = true
	}
	return selected
}

func selectionMapForAll(entries []app.PlaylistEntry) map[int]bool {
	if len(entries) == 0 {
		return nil
	}
	selected := make(map[int]bool, len(entries))
	for _, entry := range entries {
		selected[entry.Index] = true
	}
	return selected
}

func playlistEntriesFromSelection(info *app.PlaylistInfo, selected map[int]bool) []app.PlaylistEntry {
	if info == nil || len(selected) == 0 {
		return nil
	}

	entries := make([]app.PlaylistEntry, 0, len(selected))
	for _, entry := range info.Entries {
		if selected[entry.Index] {
			entries = append(entries, entry)
		}
	}
	return entries
}
