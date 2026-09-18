package core

import (
	jsonv2 "encoding/json/v2"
	"testing"
)

func TestMediaFormatUnmarshalNulls(t *testing.T) {
	var f MediaFormat
	payload := `{"height": 1080, "vcodec": null, "acodec": "mp4a", "language": null, "filesize": null, "filesize_approx": 42}`
	if err := jsonv2.Unmarshal([]byte(payload), &f); err != nil {
		t.Fatal(err)
	}
	if f.Height != 1080 || f.VCodec != "" || f.ACodec != "mp4a" || f.Language != "" || f.Filesize != 0 || f.FilesizeApprox != 42 {
		t.Fatalf("unexpected format %+v", f)
	}
}

func TestDecodeStringOr(t *testing.T) {
	if got := decodeStringOr("a", "d"); got != "a" {
		t.Fatalf("got %q", got)
	}
	if got := decodeStringOr("", "d"); got != "d" {
		t.Fatalf("got %q", got)
	}
	if got := decodeStringOr(nil, "d"); got != "d" {
		t.Fatalf("got %q", got)
	}
	if got := decodeStringOr(42, "d"); got != "d" {
		t.Fatalf("got %q", got)
	}
}

func TestDecodeFloat(t *testing.T) {
	cases := []struct {
		in   any
		want float64
	}{
		{nil, 0},
		{1.5, 1.5},
		{2, 2},
		{int64(3), 3},
		{"4.5", 4.5},
		{"  bad  ", 0},
		{true, 0},
	}
	for _, c := range cases {
		if got := decodeFloat(c.in); got != c.want {
			t.Fatalf("decodeFloat(%v)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestDecodeInt(t *testing.T) {
	if got := decodeInt("42"); got != 42 {
		t.Fatalf("got %d", got)
	}
	if got := decodeInt(3.9); got != 3 {
		t.Fatalf("got %d", got)
	}
	if got := decodeInt(nil); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestParseHelpers(t *testing.T) {
	if got := ParseIntOrZero(" 12 "); got != 12 {
		t.Fatalf("got %d", got)
	}
	if got := ParseIntOrZero("bad"); got != 0 {
		t.Fatalf("got %d", got)
	}
	if got := ParsePercentOrZero("12.5%"); got != 12.5 {
		t.Fatalf("got %v", got)
	}
	if got := ParsePercentOrZero(""); got != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestSanitizeFileStem(t *testing.T) {
	if got := SanitizeFileStem("a/b", "fb"); got != "a_b" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeFileStem("", "fb"); got != "fb" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeFileStem("", "download"); got != "download" {
		t.Fatalf("got %q", got)
	}
}
