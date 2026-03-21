package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	speed    string
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
	if elapsed := now.Sub(w.lastTime).Seconds(); elapsed > 0 {
		w.speed = FmtBytes(int64(float64(w.done-w.lastDone)/elapsed)) + "/с"
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
		Speed:  w.speed,
		Done:   fin,
		Err:    e,
	}:
	default:
	}
}

// DownloadFile загружает URL в dest и шлёт прогресс в ch (если не nil).
func DownloadFile(url, dest string, ch chan<- FileProgress) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("создание директории: %w", err)
	}
	resp, err := dlClient.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s при загрузке %s", resp.Status, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("создание файла %s: %w", dest, err)
	}
	defer f.Close()

	pw := &dlWriter{
		w:        f,
		total:    max(resp.ContentLength, 0),
		ch:       ch,
		lastTime: time.Now(),
		nextEmit: time.Now(),
	}
	_, cpErr := io.Copy(pw, resp.Body)
	if cpErr != nil {
		pw.emit(true, cpErr)
		_ = os.Remove(dest)
		return cpErr
	}
	pw.emit(true, nil)
	return nil
}
