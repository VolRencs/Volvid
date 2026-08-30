package app

import (
	"io"
	"time"
)

type FileProgress struct {
	Pct    float64
	DoneB  int64
	TotalB int64
	Speed  string
	Done   bool
	Err    error
}

type dlWriter struct {
	w        io.Writer
	total    int64
	done     int64
	lastDone int64
	lastTime time.Time
	nextEmit time.Time
	locale   Locale
	ch       chan<- FileProgress
}

func (w *dlWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.done += int64(n)
	if w.ch != nil && time.Now().After(w.nextEmit) {
		w.emit(false, nil)
		w.nextEmit = time.Now().Add(100 * time.Millisecond)
	}
	return n, err
}

func (w *dlWriter) emit(fin bool, e error) {
	if w.ch == nil {
		return
	}
	now := time.Now()
	var speed string
	if elapsed := now.Sub(w.lastTime).Seconds(); elapsed > 0 {
		speed = FmtSpeedFor(int64(float64(w.done-w.lastDone)/elapsed), w.locale)
		w.lastDone, w.lastTime = w.done, now
	}
	pct := 0.0
	if w.total > 0 {
		pct = float64(w.done) / float64(w.total) * 100
	}
	select {
	case w.ch <- FileProgress{
		Pct:    pct,
		DoneB:  w.done,
		TotalB: w.total,
		Speed:  speed,
		Done:   fin,
		Err:    e,
	}:
	default:
	}
}
