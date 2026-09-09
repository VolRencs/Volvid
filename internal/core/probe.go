package core

import "encoding/json"

type MediaProbe struct {
	Duration  int
	Formats   []MediaFormat
	HasVideo  bool
	Subtitles []SubtitleTrack
}

// SubtitleTrack is one subtitle language from the probe.
// Auto marks YouTube automatic captions as opposed to manual tracks.
type SubtitleTrack struct {
	Lang string
	Auto bool
}

type MediaFormat struct {
	Height         int    `json:"height"`
	VCodec         string `json:"vcodec"`
	ACodec         string `json:"acodec"`
	Filesize       int64  `json:"filesize"`
	FilesizeApprox int64  `json:"filesize_approx"`
}

// UnmarshalJSON tolerates null in yt-dlp string/number fields.
func (f *MediaFormat) UnmarshalJSON(data []byte) error {
	var aux struct {
		Height         any `json:"height"`
		VCodec         any `json:"vcodec"`
		ACodec         any `json:"acodec"`
		Filesize       any `json:"filesize"`
		FilesizeApprox any `json:"filesize_approx"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*f = MediaFormat{
		Height:         int(decodeInt(aux.Height)),
		VCodec:         decodeString(aux.VCodec),
		ACodec:         decodeString(aux.ACodec),
		Filesize:       decodeInt(aux.Filesize),
		FilesizeApprox: decodeInt(aux.FilesizeApprox),
	}
	return nil
}
