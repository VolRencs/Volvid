package i18n

import (
	"strings"
	"testing"

	"volvid/internal/core"
)

func TestFormatBytesLocales(t *testing.T) {
	if got := FormatBytes(2048, core.LocaleEN); got != "2 KB" {
		t.Fatalf("EN got %q", got)
	}
	if got := FormatBytes(2048, core.LocaleRU); got != "2 КБ" {
		t.Fatalf("RU got %q", got)
	}
	if got := FormatBytes(3<<20, core.LocaleEN); !strings.HasSuffix(got, "MB") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatSpeedSuffix(t *testing.T) {
	if got := FormatSpeed(2048, core.LocaleEN); !strings.HasSuffix(got, "/s") {
		t.Fatalf("got %q", got)
	}
	if got := FormatSpeed(2048, core.LocaleRU); !strings.HasSuffix(got, "/с") {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultProfilesHaveLabels(t *testing.T) {
	for _, l := range []core.Locale{core.LocaleEN, core.LocaleRU} {
		if p := DefaultVideoProfile(l); p.Label == "" || p.Mode != core.ModeVideo {
			t.Fatalf("bad default video profile for %v: %+v", l, p)
		}
		if got := len(AudioOutputProfiles(l)); got != 5 {
			t.Fatalf("expected 5 audio profiles, got %d", got)
		}
		if p := ThumbnailOutputProfile(l); p.Mode != core.ModeThumbnail || p.Label == "" {
			t.Fatalf("bad thumbnail profile: %+v", p)
		}
		if p := DefaultProfileForMode(core.ModeAudio, l); p.Mode != core.ModeAudio {
			t.Fatalf("bad audio default: %+v", p)
		}
	}
}

func TestQualityProfile(t *testing.T) {
	q := core.QualityChoice{Key: "best", Best: true, FmtChain: []string{"best"}}
	p := QualityProfile(q, core.LocaleEN)
	if p.Mode != core.ModeVideo || p.Key != "best" || p.Label == "" {
		t.Fatalf("bad quality profile: %+v", p)
	}
	if got := len(QualityChoiceLabels([]core.QualityChoice{q}, core.LocaleRU)); got != 1 {
		t.Fatalf("expected 1 label, got %d", got)
	}
}

func TestFragmentTexts(t *testing.T) {
	if got := FragmentUnavailableText(core.LocaleEN); got == "" {
		t.Fatal("expected non-empty unavailable text")
	}
	if got := FragmentDurationText(core.LocaleEN, 0); got != "" {
		t.Fatalf("expected empty text for unknown duration, got %q", got)
	}
	if got := FragmentInputHintFor(core.LocaleRU, 90); !strings.Contains(got, "01:30") {
		t.Fatalf("expected hint with 01:30, got %q", got)
	}
	if got := FragmentInputErrorText(core.LocaleEN, nil, 0); got != "" {
		t.Fatalf("expected empty text for nil error, got %q", got)
	}
	if got := FragmentInputErrorText(core.LocaleEN, core.ErrFragmentFormat, 0); got == "" {
		t.Fatal("expected non-empty text for format error")
	}
}

func TestStringsFor(t *testing.T) {
	if StringsFor(core.LocaleEN).SearchPlaceholder == "" {
		t.Fatal("empty EN placeholder")
	}
	if StringsFor(core.LocaleRU).SearchPlaceholder == "" {
		t.Fatal("empty RU placeholder")
	}
	if got := PlaylistSuffix(core.LocaleEN, 0); got != "" {
		t.Fatalf("expected empty suffix for 0, got %q", got)
	}
	if got := PlaylistSuffix(core.LocaleEN, 3); !strings.Contains(got, "3") {
		t.Fatalf("expected suffix with 3, got %q", got)
	}
}
