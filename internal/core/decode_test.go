package core

import "testing"

func TestDecodeStringOr(t *testing.T) {
	if got := DecodeStringOr("a", "d"); got != "a" {
		t.Fatalf("got %q", got)
	}
	if got := DecodeStringOr("", "d"); got != "d" {
		t.Fatalf("got %q", got)
	}
	if got := DecodeStringOr(nil, "d"); got != "d" {
		t.Fatalf("got %q", got)
	}
	if got := DecodeStringOr(42, "d"); got != "d" {
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
		if got := DecodeFloat(c.in); got != c.want {
			t.Fatalf("DecodeFloat(%v)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestDecodeInt(t *testing.T) {
	if got := DecodeInt("42"); got != 42 {
		t.Fatalf("got %d", got)
	}
	if got := DecodeInt(3.9); got != 3 {
		t.Fatalf("got %d", got)
	}
	if got := DecodeInt(nil); got != 0 {
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
