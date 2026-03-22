package app

type SessionItem struct {
	Label string
	URL   string
	OK    bool
}

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
