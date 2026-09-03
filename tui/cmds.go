package tui

import (
	"context"
	"time"

	app "volvid/internal/app"

	tea "charm.land/bubbletea/v2"
)

const (
	spinnerTickInterval  = 90 * time.Millisecond
	digitTimeoutInterval = 700 * time.Millisecond
)

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(spinnerTickInterval, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func timerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(ts time.Time) tea.Msg { return timerTickMsg(ts) })
}

func digitTimeoutCmd() tea.Cmd {
	return tea.Tick(digitTimeoutInterval, func(time.Time) tea.Msg { return menuDigitTickMsg{} })
}

func openDownloadsDirCmd(path string) tea.Cmd {
	return func() tea.Msg {
		return msgOpenDownloadsDirDone{err: app.OpenInFileManager(path)}
	}
}

func pickDownloadsDirCmd(env *app.Env, path string, locale app.Locale) tea.Cmd {
	return func() tea.Msg {
		dir, err := app.PickDownloadsDir(env, path, locale)
		return msgPickDownloadsDirDone{path: dir, err: err}
	}
}

func streamFileProgressCmd(ch <-chan app.FileProgress, isUpdate bool) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgDepDone{isUpdate: isUpdate}
		}
		if p.Done {
			return msgDepDone{err: p.Err, isUpdate: isUpdate}
		}
		return msgDepProgress{progress: p}
	}
}

func launchProgress(
	base context.Context,
	fn func(context.Context, chan<- app.FileProgress) error,
	isUpdate bool,
) (<-chan app.FileProgress, tea.Cmd, context.CancelFunc) {
	ch := make(chan app.FileProgress, 16)
	ctx, cancel := context.WithCancel(base)

	go func() {
		defer close(ch)

		progressCh := make(chan app.FileProgress, 16)
		doneCh := make(chan error, 1)

		go func() {
			defer close(progressCh)
			doneCh <- fn(ctx, progressCh)
		}()

		for progress := range progressCh {
			select {
			case <-ctx.Done():
				return
			default:
			}
			progress.Done = false
			progress.Err = nil
			ch <- progress
		}

		if err := <-doneCh; err != nil {
			if ctx.Err() != nil {
				err = context.Canceled
			}
			ch <- app.FileProgress{Done: true, Err: err}
			return
		}

		ch <- app.FileProgress{Done: true}
	}()

	return ch, streamFileProgressCmd(ch, isUpdate), cancel
}

func refreshDepsCmd(env *app.Env, token int) tea.Cmd {
	return func() tea.Msg {
		return msgDepsRefreshed{deps: app.RefreshDeps(env), token: token}
	}
}

func fetchPlaylistCmd(env *app.Env, ctx context.Context, url string, l app.Locale, gen int) tea.Cmd {
	return func() tea.Msg {
		info, err := app.FetchPlaylistInfoFor(env, ctx, url, l)
		return msgPlaylistFetched{info: info, err: err, gen: gen}
	}
}

func searchYouTubeCmd(env *app.Env, ctx context.Context, query string, gen int) tea.Cmd {
	return func() tea.Msg {
		results, err := app.SearchYouTubeContext(env, ctx, query)
		return msgSearchResults{results: results, err: err, gen: gen}
	}
}

func loadQualityChoicesCmd(env *app.Env, ctx context.Context, urls []string, gen int) tea.Cmd {
	return func() tea.Msg {
		choices, err := app.ResolveQualityChoicesContext(env, ctx, urls)
		return msgQualityScanned{choices: choices, err: err, gen: gen}
	}
}

func probeFragmentDurationCmd(env *app.Env, ctx context.Context, target app.ParsedTarget, gen int) tea.Cmd {
	return func() tea.Msg {
		duration, err := app.ProbeMediaDurationContext(env, ctx, target)
		return msgFragmentDuration{duration: duration, err: err, gen: gen}
	}
}

func listenDownloadCmd(ch <-chan app.DlUpdate, gen int) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return msgDlUpdate{update: app.DlUpdate{Type: app.EvClosed}, gen: gen}
		}
		return msgDlUpdate{update: u, gen: gen}
	}
}

func checkUpdateCmd(env *app.Env) tea.Cmd {
	return func() tea.Msg {
		return msgUpdateChecked{info: app.CheckUpdate(env)}
	}
}
