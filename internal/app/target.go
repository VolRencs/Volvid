package app

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

type TargetKind uint8

const (
	TargetUnknown TargetKind = iota
	TargetVideo
	TargetPlaylist
	TargetMixed
)

type ParsedTarget struct {
	CanonicalURL string
	Kind         TargetKind
	VideoID      string
	PlaylistID   string
	URLStartAt   int
	HasURLStart  bool
}

func (k TargetKind) String() string {
	switch k {
	case TargetVideo:
		return "video"
	case TargetPlaylist:
		return "playlist"
	case TargetMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

func (t ParsedTarget) IsVideo() bool {
	return t.Kind == TargetVideo || t.Kind == TargetMixed
}

func (t ParsedTarget) IsPlaylist() bool {
	return t.Kind == TargetPlaylist || t.Kind == TargetMixed
}

func (t ParsedTarget) VideoURL() string {
	if t.VideoID == "" {
		return t.CanonicalURL
	}
	return "https://www.youtube.com/watch?v=" + url.QueryEscape(t.VideoID)
}

func (t ParsedTarget) PlaylistURL() string {
	if t.PlaylistID == "" {
		return t.CanonicalURL
	}
	return "https://www.youtube.com/playlist?list=" + url.QueryEscape(t.PlaylistID)
}

func (t ParsedTarget) DownloadURL(forceSingle bool) string {
	if forceSingle && t.VideoID != "" {
		return t.VideoURL()
	}
	return t.CanonicalURL
}

func ParseTarget(raw string) (ParsedTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedTarget{}, fmt.Errorf("empty URL")
	}

	normalized := raw
	if !strings.Contains(normalized, "://") {
		normalized = "https://" + normalized
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return ParsedTarget{}, err
	}

	host := normalizeYouTubeHost(u.Hostname())
	if host == "" {
		return ParsedTarget{}, fmt.Errorf("unsupported host %q", u.Hostname())
	}

	target := ParsedTarget{}
	if err := parseTargetIDs(&target, host, u); err != nil {
		return ParsedTarget{}, err
	}
	target.URLStartAt, target.HasURLStart = ParseURLStartAt(raw)
	target.CanonicalURL = canonicalTargetURL(target)
	return target, nil
}

func normalizeYouTubeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	host = strings.TrimPrefix(host, "music.")

	switch host {
	case "youtube.com", "youtu.be":
		return host
	default:
		return ""
	}
}

func parseTargetIDs(target *ParsedTarget, host string, u *url.URL) error {
	query := u.Query()
	switch host {
	case "youtu.be":
		target.VideoID = cleanTargetID(u.Path)
		target.PlaylistID = strings.TrimSpace(query.Get("list"))
	case "youtube.com":
		switch cleanTargetID(u.Path) {
		case "watch":
			target.VideoID = strings.TrimSpace(query.Get("v"))
			target.PlaylistID = strings.TrimSpace(query.Get("list"))
		case "playlist":
			target.PlaylistID = strings.TrimSpace(query.Get("list"))
		default:
			parts := splitPath(u.Path)
			if len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "live") {
				target.VideoID = parts[1]
				target.PlaylistID = strings.TrimSpace(query.Get("list"))
			}
		}
	}

	switch {
	case target.VideoID != "" && target.PlaylistID != "":
		target.Kind = TargetMixed
	case target.VideoID != "":
		target.Kind = TargetVideo
	case target.PlaylistID != "":
		target.Kind = TargetPlaylist
	default:
		return fmt.Errorf("unsupported YouTube URL")
	}
	return nil
}

func canonicalTargetURL(target ParsedTarget) string {
	switch target.Kind {
	case TargetPlaylist:
		return "https://www.youtube.com/playlist?list=" + url.QueryEscape(target.PlaylistID)
	case TargetMixed:
		values := url.Values{}
		values.Set("v", target.VideoID)
		values.Set("list", target.PlaylistID)
		return "https://www.youtube.com/watch?" + values.Encode()
	default:
		return "https://www.youtube.com/watch?v=" + url.QueryEscape(target.VideoID)
	}
}

func splitPath(raw string) []string {
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}

func cleanTargetID(raw string) string {
	value := strings.Trim(path.Clean(raw), "/")
	if value == "." {
		return ""
	}
	return value
}
