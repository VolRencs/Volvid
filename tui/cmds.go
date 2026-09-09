package tui

import (
	"context"
	"time"
	"volvid/internal/core"

	"volvid/internal/services"

	tea "charm.land/bubbletea/v2"
)

const (
	spinnerTickInterval  = 90 * time.Millisecond
	timerTickInterval    = time.Second
	digitTimeoutInterval = 700 * time.Millisecond
)

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(spinnerTickInterval, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func timerTickCmd() tea.Cmd {
	return tea.Tick(timerTickInterval, func(ts time.Time) tea.Msg { return timerTickMsg(ts) })
}

func digitTimeoutCmd() tea.Cmd {
	return tea.Tick(digitTimeoutInterval, func(time.Time) tea.Msg { return menuDigitTickMsg{} })
}

func openDownloadsDirCmd(api AppAPI, path string) tea.Cmd {
	return func() tea.Msg {
		return msgOpenDownloadsDirDone{err: api.OpenFolder(path)}
	}
}

func pickDownloadsDirCmd(ctx context.Context, api AppAPI, path string, locale core.Locale, gen int) tea.Cmd {
	return func() tea.Msg {
		dir, err := api.PickDir(ctx, path, locale)
		return msgPickDownloadsDirDone{path: dir, err: err, gen: gen}
	}
}

func streamFileProgressCmd(ch <-chan core.FileProgress, isUpdate bool, gen int) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgDepDone{isUpdate: isUpdate, gen: gen}
		}
		if p.Done {
			return msgDepDone{err: p.Err, isUpdate: isUpdate, gen: gen}
		}
		return msgDepProgress{progress: p, gen: gen}
	}
}

// launchProgress is a thin tea.Cmd wrapper around the service-layer worker.
// Concurrency policy (double channel, panic recovery, exactly-once terminal
// message) lives in internal/services/progress.go and is unit-testable
// without Bubble Tea.
func launchProgress(
	base context.Context,
	fn func(context.Context, chan<- core.FileProgress) error,
	isUpdate bool,
	gen int,
) (<-chan core.FileProgress, tea.Cmd, context.CancelFunc) {
	ch, cancel := services.LaunchProgress(base, fn)
	return ch, streamFileProgressCmd(ch, isUpdate, gen), cancel
}

func refreshDepsCmd(api AppAPI, token int) tea.Cmd {
	return func() tea.Msg {
		return msgDepsRefreshed{deps: api.RefreshDeps(), token: token}
	}
}

func fetchPlaylistCmd(api AppAPI, ctx context.Context, url string, l core.Locale, gen int) tea.Cmd {
	return func() tea.Msg {
		info, err := api.FetchPlaylist(ctx, url, l)
		return msgPlaylistFetched{info: info, err: err, gen: gen}
	}
}

func searchYouTubeCmd(api AppAPI, ctx context.Context, query string, gen int) tea.Cmd {
	return func() tea.Msg {
		results, err := api.SearchYouTube(ctx, query)
		return msgSearchResults{results: results, err: err, gen: gen}
	}
}

func loadQualityChoicesCmd(api AppAPI, ctx context.Context, urls []string, gen int) tea.Cmd {
	return func() tea.Msg {
		choices, err := api.ResolveQuality(ctx, urls)
		return msgQualityScanned{choices: choices, err: err, gen: gen}
	}
}

func loadTracksCmd(api AppAPI, ctx context.Context, url string, gen int) tea.Cmd {
	return func() tea.Msg {
		audioTracks, audioErr := api.ResolveAudioTracks(ctx, url)
		subTracks, subErr := api.ResolveSubtitles(ctx, url)
		return msgTracksLoaded{
			audioTracks: audioTracks,
			audioErr:    audioErr,
			subTracks:   subTracks,
			subErr:      subErr,
			gen:         gen,
		}
	}
}

func probeFragmentDurationCmd(api AppAPI, ctx context.Context, target core.ParsedTarget, gen int) tea.Cmd {
	return func() tea.Msg {
		duration, err := api.ProbeDuration(ctx, target)
		return msgFragmentDuration{duration: duration, err: err, gen: gen}
	}
}

func listenDownloadCmd(ch <-chan core.DlUpdate, gen int) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return msgDlUpdate{update: core.DlUpdate{Type: core.EvClosed}, gen: gen}
		}
		return msgDlUpdate{update: u, gen: gen}
	}
}

func checkUpdateCmd(api AppAPI) tea.Cmd {
	return func() tea.Msg {
		return msgUpdateChecked{info: api.CheckUpdate()}
	}
}
