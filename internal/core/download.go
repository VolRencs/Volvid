package core

import "strings"

type DlEventType uint8

const (
	EvStart DlEventType = iota
	EvDest
	EvProgress
	EvProc
	EvDone
	EvReset
	EvFallback
	EvClosed
)

type DlUpdate struct {
	Type    DlEventType
	Slot    int
	Text    string
	ErrText string
	Pct     float64
	DoneB   int64
	TotalB  int64
	Speed   string
	OK      bool
}

// FileProgress reports managed-binary / update downloads.
type FileProgress struct {
	Pct    float64
	DoneB  int64
	TotalB int64
	Speed  string
	Done   bool
	Err    error
}

// FFmpegArgs renders the shared --ffmpeg-location prefix for yt-dlp.
func FFmpegArgs(deps CheckDepsResult) []string {
	bin := strings.TrimSpace(deps.FFmpeg.Path)
	if bin == "" {
		return nil
	}
	return []string{"--ffmpeg-location", bin}
}
