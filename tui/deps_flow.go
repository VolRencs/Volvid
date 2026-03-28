package tui

import (
	"fmt"

	app "YouTubeBuild/internal/app"

	tea "charm.land/bubbletea/v2"
)

func (m Model) gotoChecks() (tea.Model, tea.Cmd) {
	deps := app.DetectDeps()
	m = m.withDeps(deps)
	if deps.MissingRequired() {
		return m.openDependencyScreen(depModeStartup)
	}
	return m.gotoURLWithDeps(deps)
}

func (m Model) gotoURL() (tea.Model, tea.Cmd) {
	return m.gotoURLWithDeps(app.DetectDeps())
}

func (m Model) gotoURLWithDeps(deps app.CheckDepsResult) (tea.Model, tea.Cmd) {
	m = m.withDeps(deps)
	m.screen = scrURL
	return m, m.urlInput.Focus()
}

func (m Model) withDeps(deps app.CheckDepsResult) Model {
	m.deps = deps
	return m
}

func (m Model) openDependencyScreen(mode depScreenMode) (tea.Model, tea.Cmd) {
	return m.openDependencyScreenWithError(mode, "")
}

func (m Model) openDependencyScreenWithError(mode depScreenMode, errText string) (tea.Model, tea.Cmd) {
	m.depMode = mode
	m.depErr = errText
	m.screen = scrDepUpdate
	m = m.syncMenu()
	return m, nil
}

func (m Model) startDepUpdate() (tea.Model, tea.Cmd) {
	m.prevScreen = m.screen
	m.depUpdateDone = false
	return m.openDependencyScreen(depModeManage)
}

func (m Model) depActions() []depAction {
	actions := make([]depAction, 0, 5)
	for _, dep := range m.deps.ActionableDependencies() {
		label := dep.Name
		switch {
		case dep.Available && dep.Source == app.DepManaged:
			label = m.depActionLabel("update", dep.Name)
		default:
			label = m.depActionLabel("download", dep.Name)
		}
		actions = append(actions, depAction{Kind: depActionInstall, Key: dep.Key, Label: label})
	}

	actions = append(actions, depAction{Kind: depActionRefresh, Label: m.depActionLabel("refresh", "")})
	if !m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionContinue, Label: m.depActionLabel("continue", "")})
	}
	if m.depMode == depModeManage {
		actions = append(actions, depAction{Kind: depActionBack, Label: m.depActionLabel("back", "")})
	} else if m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionExit, Label: m.depActionLabel("exit", "")})
	}
	return actions
}

func (m Model) startDependencyDownload(
	screen screen,
	label string,
	isUpdate bool,
	fn func(chan<- app.FileProgress) error,
) (tea.Model, tea.Cmd) {
	m.depLabel = label
	m.depProgress = app.FileProgress{}
	m.depErr = ""
	m.screen = screen

	var cmd tea.Cmd
	m.depCh, cmd = launchProgress(fn, isUpdate)
	return m, cmd
}

func (m Model) depActionLabel(kind, name string) string {
	if m.locale == app.LocaleRU {
		switch kind {
		case "download":
			return fmt.Sprintf("Скачать %s", name)
		case "update":
			return fmt.Sprintf("Обновить %s", name)
		case "refresh":
			return "Обновить статус"
		case "continue":
			return "Продолжить"
		case "back":
			return "Назад"
		case "exit":
			return "Выход"
		}
	}

	switch kind {
	case "download":
		return fmt.Sprintf("Download %s", name)
	case "update":
		return fmt.Sprintf("Update %s", name)
	case "refresh":
		return "Refresh status"
	case "continue":
		return "Continue"
	case "back":
		return "Back"
	case "exit":
		return "Exit"
	}
	return name
}

func (m Model) depRequirementText(name string) string {
	if m.locale == app.LocaleRU {
		return fmt.Sprintf("%s требуется для этого режима.", name)
	}
	return fmt.Sprintf("%s is required for this mode.", name)
}
