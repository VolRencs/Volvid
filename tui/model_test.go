package tui

import (
	"context"
	"testing"

	"volvid/internal/adapters"
	"volvid/internal/core"
)

func newTestModel() Model {
	return newModelWithAPI(context.Background(), newAppAPI(adapters.NewEnv()))
}

func TestDepsVersionsTokenGuard(t *testing.T) {
	stub := newStubAPI()
	m := newStubModel(stub)
	m.depRefreshToken = 7

	stale := core.CheckDepsResult{YTDLP: core.DependencyInfo{Key: "ytdlp", Version: "stale"}}
	model, _ := m.Update(msgDepsVersions{deps: stale, token: 6})
	if got := model.(Model).deps.YTDLP.Version; got != "" {
		t.Fatalf("stale versions must be ignored, got %q", got)
	}

	fresh := core.CheckDepsResult{YTDLP: core.DependencyInfo{Key: "ytdlp", Version: "fresh"}}
	model, _ = m.Update(msgDepsVersions{deps: fresh, token: 7})
	if got := model.(Model); got.deps.YTDLP.Version != "fresh" {
		t.Fatalf("expected fresh versions, got %q", got.deps.YTDLP.Version)
	} else if got.depRefreshing {
		t.Fatal("depRefreshing must clear once versions arrive")
	}
}

func TestDepsCheckedStartsVersionEnrichment(t *testing.T) {
	stub := newStubAPI()
	m := newStubModel(stub)

	model, cmd := m.Update(msgDepsChecked{deps: testDeps(true, true)})
	if cmd == nil {
		t.Fatal("expected an enrichment command")
	}
	if !model.(Model).depRefreshing {
		t.Fatal("expected depRefreshing while versions are pending")
	}
}

func TestOpenDependencyScreenUsesKnownVersions(t *testing.T) {
	stub := newStubAPI()
	m := newStubModel(stub)
	m.screen = scrURL
	m.deps.YTDLP.Version = "2026.09.18"

	model, cmd := m.startDepUpdate()
	got := model.(Model)
	if cmd != nil {
		t.Fatal("opening Ctrl+U must not force a dependency refresh")
	}
	if got.screen != scrDepUpdate {
		t.Fatalf("expected dep screen, got %v", got.screen)
	}
	if got.deps.YTDLP.Version != "2026.09.18" {
		t.Fatalf("known version must be preserved, got %q", got.deps.YTDLP.Version)
	}
}
