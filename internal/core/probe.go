package core

import "encoding/json"

type MediaProbe struct {
	Duration int
	Formats  []MediaFormat
	HasVideo bool
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
		Height         int `json:"height"`
		VCodec         any `json:"vcodec"`
		ACodec         any `json:"acodec"`
		Filesize       any `json:"filesize"`
		FilesizeApprox any `json:"filesize_approx"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*f = MediaFormat{
		Height:         aux.Height,
		VCodec:         decodeString(aux.VCodec),
		ACodec:         decodeString(aux.ACodec),
		Filesize:       decodeInt(aux.Filesize),
		FilesizeApprox: decodeInt(aux.FilesizeApprox),
	}
	return nil
}
