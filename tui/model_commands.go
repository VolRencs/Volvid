package tui

import (
	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

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

func launchProgress(fn func(chan<- app.FileProgress) error, isUpdate bool) (<-chan app.FileProgress, tea.Cmd) {
	ch := make(chan app.FileProgress, 16)
	go func() {
		defer close(ch)

		progressCh := make(chan app.FileProgress, 16)
		doneCh := make(chan error, 1)

		go func() {
			defer close(progressCh)
			doneCh <- fn(progressCh)
		}()

		for progress := range progressCh {
			progress.Done = false
			progress.Err = nil
			ch <- progress
		}

		if err := <-doneCh; err != nil {
			ch <- app.FileProgress{Done: true, Err: err}
			return
		}

		ch <- app.FileProgress{Done: true}
	}()
	return ch, streamFileProgressCmd(ch, isUpdate)
}

func fetchPlaylistCmd(url string, l app.Locale) tea.Cmd {
	return func() tea.Msg {
		info, err := app.FetchPlaylistInfoFor(nil, url, l)
		return msgPlaylistFetched{info: info, err: err}
	}
}

func searchYouTubeCmd(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := app.SearchYouTube(query)
		return msgSearchResults{results: results, err: err}
	}
}

func loadQualityChoicesCmd(urls []string) tea.Cmd {
	return func() tea.Msg {
		choices, err := app.ResolveQualityChoices(urls)
		return msgQualityScanned{choices: choices, err: err}
	}
}

func probeFragmentDurationCmd(target app.ParsedTarget) tea.Cmd {
	return func() tea.Msg {
		duration, err := app.ProbeMediaDuration(target)
		return msgFragmentDuration{duration: duration, err: err}
	}
}

func listenDownloadCmd(ch <-chan app.DlUpdate) tea.Cmd {
	return func() tea.Msg {
		u, ok := <-ch
		if !ok {
			return msgDlUpdate{update: app.DlUpdate{Type: app.EvClosed}}
		}
		return msgDlUpdate{update: u}
	}
}
