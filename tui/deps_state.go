package tui

import (
	"fmt"
	"volvid/internal/core"
	"volvid/internal/i18n"
)

func (m Model) depActions() []depAction {
	actions := make([]depAction, 0, 5)
	for _, dep := range m.deps.ActionableDependencies() {
		label := depActionText(depActionInstall, dep.Name, dep.Available && dep.Source == core.DepManaged, m.u())
		actions = append(actions, depAction{Kind: depActionInstall, Key: dep.Key, Label: label})
	}

	if !m.depRefreshing {
		actions = append(actions, depAction{Kind: depActionRefresh, Label: depActionText(depActionRefresh, "", false, m.u())})
	}
	if m.depMode == depModeStartup && !m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionContinue, Label: depActionText(depActionContinue, "", false, m.u())})
	}
	if m.depMode == depModeManage {
		actions = append(actions, depAction{Kind: depActionBack, Label: depActionText(depActionBack, "", false, m.u())})
	} else if m.deps.MissingRequired() {
		actions = append(actions, depAction{Kind: depActionExit, Label: depActionText(depActionExit, "", false, m.u())})
	}
	return actions
}
func depActionText(kind depActionKind, name string, isUpdate bool, u *i18n.UIStrings) string {
	if kind == depActionInstall {
		if isUpdate {
			return fmt.Sprintf(u.DepActionUpdateFmt, name)
		}
		return fmt.Sprintf(u.DepActionDownloadFmt, name)
	}
	switch kind {
	case depActionRefresh:
		return u.DepActionRefresh
	case depActionContinue:
		return u.DepActionContinue
	case depActionBack:
		return u.DepActionBack
	case depActionExit:
		return u.DepActionExit
	}
	return name
}
func (m Model) depRequirementText(name string) string {
	return fmt.Sprintf(m.u().DepRequirementFmt, name)
}
