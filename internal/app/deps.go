package app

import (
	"fmt"
	"strings"
	"time"
)

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
	Status  string
	Browser string
	Profile string
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
	versionProbeAttempts = 8
	versionProbeDelay    = 250 * time.Millisecond
	versionProbeTimeout  = 5 * time.Second
)

type DepsLogger func(format string, args ...any)

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

func (r CheckDepsResult) DownloadableMissing() []DependencyInfo {
	return filterDependencies(r.Dependencies(), func(dep DependencyInfo) bool {
		return !dep.Available && dep.Downloadable
	})
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

func EnsureRuntimeDeps(logf DepsLogger) (CheckDepsResult, error) {
	deps := DetectDeps()
	for _, dep := range deps.DownloadableMissing() {
		if logf != nil {
			logf("Зависимости: %s не найден, скачиваю…", dep.Name)
		}
		if err := InstallDependencyFor(dep.Key, LocaleEN, nil); err != nil {
			if dep.Required {
				return DetectDeps(), fmt.Errorf("установка %s: %w", dep.Name, err)
			}
			if logf != nil {
				logf("Зависимости: не удалось подготовить %s: %v", dep.Name, err)
			}
		}
	}

	deps = DetectDeps()
	if deps.MissingRequired() {
		return deps, fmt.Errorf("%s не найден", strings.Join(missingDependencyNames(deps.MissingRequiredDeps()), ", "))
	}
	return deps, nil
}

func UpdateManagedDeps(ch chan<- FileProgress) error {
	return UpdateManagedDepsFor(LoadLocale(), ch)
}

func UpdateManagedDepsFor(l Locale, ch chan<- FileProgress) error {
	deps := DetectDeps()
	for _, dep := range deps.ActionableDependencies() {
		if dep.Source == DepSystem {
			continue
		}
		if err := InstallDependencyFor(dep.Key, l, ch); err != nil {
			return fmt.Errorf("%s: %w", dep.Name, err)
		}
	}
	return nil
}

func missingDependencyNames(deps []DependencyInfo) []string {
	names := make([]string, 0, len(deps))
	for _, dep := range deps {
		name := strings.TrimSpace(dep.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}
