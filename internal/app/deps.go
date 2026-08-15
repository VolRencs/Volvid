package app

import "time"

type DependencySource string

const (
	DepMissing DependencySource = "missing"
	DepSystem  DependencySource = "system"
	DepManaged DependencySource = "_deps"
)

type DependencyInfo struct {
	Key          string
	Name         string
	Path         string
	Version      string
	Source       DependencySource
	Required     bool
	Downloadable bool
	Managed      bool
	Available    bool
}

type BrowserCookiesInfo struct {
	Status       string
	Browser      string
	Profile      string
	ProfileName  string
	CookiePath   string
	YTDLPProfile string
}

type JSRuntimeInfo struct {
	Status string
	Name   string
	Path   string
}

type CheckDepsResult struct {
	YTDLP   DependencyInfo
	FFmpeg  DependencyInfo
	Node    DependencyInfo
	Cookies BrowserCookiesInfo
	Runtime JSRuntimeInfo
}

const (
	versionProbeTimeout = 1500 * time.Millisecond
)

func (r CheckDepsResult) Dependencies() []DependencyInfo {
	return []DependencyInfo{r.YTDLP, r.FFmpeg, r.Node}
}

func (r CheckDepsResult) MissingRequiredDeps() []DependencyInfo {
	return filterDependencies(r.Dependencies(), func(dep DependencyInfo) bool {
		return dep.Required && !dep.Available
	})
}

func (r CheckDepsResult) MissingRequired() bool {
	return len(r.MissingRequiredDeps()) > 0
}

func (r CheckDepsResult) ActionableDependencies() []DependencyInfo {
	return filterDependencies(r.Dependencies(), func(dep DependencyInfo) bool {
		return dep.Downloadable && (!dep.Available || dep.Source == DepManaged)
	})
}

func filterDependencies(deps []DependencyInfo, keep func(DependencyInfo) bool) []DependencyInfo {
	out := make([]DependencyInfo, 0, len(deps))
	for _, dep := range deps {
		if keep(dep) {
			out = append(out, dep)
		}
	}
	return out
}