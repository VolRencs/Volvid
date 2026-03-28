package bot

import (
	"errors"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) buildDownloadRequest(sess *Session) (app.DownloadRequest, error) {
	snap := sess.snapshot()
	profile := snap.Profile
	if profile.Mode == 0 {
		profile = app.DefaultProfileForMode(snap.Mode, app.LocaleRU)
	}

	entries, err := b.requestEntries(sess, snap)
	if err != nil {
		return app.DownloadRequest{}, err
	}

	return app.PrepareDownloadRequest(app.DownloadRequest{
		Target:       snap.Target,
		Profile:      profile,
		Fragment:     snap.Fragment,
		ForceSingle:  snap.ForceSingle,
		PlaylistInfo: snap.PlInfo,
		Entries:      entries,
		Workers:      app.AutoDownloadWorkers(len(entries)),
		OutputDir:    snap.WorkDir,
		Locale:       app.LocaleRU,
	})
}

func (b *Bot) requestEntries(sess *Session, snap SessionSnapshot) ([]app.PlaylistEntry, error) {
	entries := b.playlistSelectionEntries(sess)
	if snap.PlInfo == nil || snap.ForceSingle {
		return entries, nil
	}
	if len(entries) == 0 {
		return nil, errors.New("Сначала выбери хотя бы одно видео из плейлиста.")
	}
	if !b.validatePlaylistSelectionCount(snap.UserID, len(entries)) {
		return nil, errors.New(b.playlistItemLimitAlert(snap.UserID))
	}
	return entries, nil
}
