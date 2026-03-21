package main

import "regexp"

var (
	YtRE = regexp.MustCompile(
		`(?i)(youtube\.com/(watch\?.*v=|shorts/|live/|playlist\?list=)|youtu\.be/)[\w\-]+`,
	)
	PlaylistRE = regexp.MustCompile(
		`(?i)(youtube\.com/playlist\?|[?&]list=[\w\-]{10,})`,
	)
	VideoInPlaylistRE = regexp.MustCompile(
		`(?i)youtube\.com/watch\?.*v=[\w\-]{11}.*[?&]list=`,
	)
)

func IsPlaylistURL(url string) bool { return PlaylistRE.MatchString(url) }
