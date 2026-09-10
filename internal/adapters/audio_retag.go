package adapters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// iso639_2By1 maps ISO 639-1 two-letter codes to ISO 639-2/T three-letter
// codes used for embedded audio stream metadata.
var iso639_2By1 = map[string]string{
	"en": "eng", "ru": "rus", "de": "deu", "fr": "fra", "es": "spa",
	"it": "ita", "pt": "por", "uk": "ukr", "be": "bel", "pl": "pol",
	"cs": "ces", "sk": "slk", "bg": "bul", "sr": "srp", "hr": "hrv",
	"bs": "bos", "mk": "mkd", "sl": "slv", "ro": "ron", "hu": "hun",
	"el": "ell", "tr": "tur", "ar": "ara", "he": "heb", "fa": "fas",
	"hi": "hin", "bn": "ben", "ta": "tam", "te": "tel", "mr": "mar",
	"ml": "mal", "pa": "pan", "ur": "urd", "id": "ind", "ms": "msa",
	"vi": "vie", "th": "tha", "ko": "kor", "ja": "jpn", "zh": "zho",
	"nl": "nld", "sv": "swe", "no": "nor", "da": "dan", "fi": "fin",
	"et": "est", "lv": "lav", "lt": "lit", "ka": "kat", "hy": "hye",
	"az": "aze", "kk": "kaz", "uz": "uzb", "ky": "kir", "tg": "tgk",
	"mn": "mon", "ne": "nep", "si": "sin", "km": "khm", "lo": "lao",
	"my": "mya", "sw": "swa", "am": "amh", "zu": "zul", "af": "afr",
	"eu": "eus", "ca": "cat", "gl": "glg",
}

// audioLangISO maps a BCP-47 probe tag ("ru", "en-US", "zh-Hans") to an
// ISO 639-2 code for stream metadata. Unknown tags report false and are
// skipped by the retag pass.
func audioLangISO(lang string) (string, bool) {
	tag := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(lang), "_", "-"))
	if tag == "" || tag == "all" {
		return "", false
	}
	primary := tag
	if i := strings.IndexByte(tag, '-'); i >= 0 {
		primary = tag[:i]
	}
	if len(primary) == 3 && isASCIILetters(primary) {
		return primary, true
	}
	if iso, ok := iso639_2By1[primary]; ok {
		return iso, true
	}
	return "", false
}

func isASCIILetters(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

// audioRetagArgs builds a stream-copy ffmpeg command that stamps ISO
// language tags onto audio streams in order. An empty slot means the
// language is unmappable: it is skipped, but the stream index is preserved
// so later tags stay on their own tracks. faststart keeps mp4 layout.
func audioRetagArgs(input, output string, isoLangs []string, faststart bool) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", input,
		"-map", "0",
		"-c", "copy",
	}
	if faststart {
		args = append(args, "-movflags", "+faststart")
	}
	for i, iso := range isoLangs {
		if iso == "" {
			continue
		}
		args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), "language="+iso)
	}
	return append(args, output)
}

// retagAudioLanguages stamps ISO language tags onto the embedded audio
// streams (in selection order) via a fast stream-copy remux. yt-dlp merge
// drops source language tags, so without this players list every track as
// "English". A retag failure never fails the download: tracks stay embedded,
// only labels may be wrong.
func retagAudioLanguages(ctx context.Context, ffmpeg, videoPath string, langs []string) error {
	videoPath = strings.TrimSpace(videoPath)
	ffmpeg = strings.TrimSpace(ffmpeg)
	if videoPath == "" || ffmpeg == "" {
		return fmt.Errorf("retag requires ffmpeg and video path")
	}
	isoLangs := make([]string, len(langs))
	mappable := 0
	for i, lang := range langs {
		if iso, ok := audioLangISO(lang); ok {
			isoLangs[i] = iso
			mappable++
		}
	}
	if mappable == 0 {
		return fmt.Errorf("no mappable audio languages")
	}

	dir := filepath.Dir(videoPath)
	base := filepath.Base(videoPath)
	ext := strings.TrimPrefix(filepath.Ext(base), ".")
	tmp, err := os.CreateTemp(dir, "."+base+".retag-*."+ext)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Close()

	faststart := strings.EqualFold(ext, "mp4")
	args := audioRetagArgs(videoPath, tmpName, isoLangs, faststart)
	if out, err := commandCombinedOutput(ctx, 0, ffmpeg, args...); err != nil {
		_ = os.Remove(tmpName)
		if text := strings.TrimSpace(string(out)); text != "" {
			return fmt.Errorf("retag audio languages: %s: %w", text, err)
		}
		return fmt.Errorf("retag audio languages: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := replaceDownloadedFile(tmpName, videoPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("retag audio languages: %w", err)
	}
	return nil
}
