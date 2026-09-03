package app

type SessionItem struct {
	Label string
	URL   string
	OK    bool
}

// Session accumulates per-run download history. It must be used from the
// UI goroutine only: Bubble Tea runs Update and View sequentially, so no
// locking is needed — but never touch it from engine goroutines.
type Session struct {
	Success int
	Failed  int
	Items   []SessionItem
}

func (s *Session) Record(label, url string, ok bool) {
	if ok {
		s.Success++
	} else {
		s.Failed++
	}
	s.Items = append(s.Items, SessionItem{Label: label, URL: url, OK: ok})
}
