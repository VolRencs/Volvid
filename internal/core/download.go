package core

import (
	"strings"
)

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

// DlUpdate is a download engine event (single file or playlist slot).
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

// FFmpegLocation returns the ffmpeg binary path from deps.
func FFmpegLocation(deps CheckDepsResult) string {
	return strings.TrimSpace(deps.FFmpeg.Path)
}

// FFmpegArgs renders the shared --ffmpeg-location prefix for yt-dlp.
func FFmpegArgs(deps CheckDepsResult) []string {
	if bin := FFmpegLocation(deps); bin != "" {
		return []string{"--ffmpeg-location", bin}
	}
	return nil
}
