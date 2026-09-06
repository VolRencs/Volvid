package tui

import (
	"strings"
)

type binding struct {
	key  string
	help string
}

func (m Model) kbMove() binding   { return binding{key: "↑/↓", help: m.u().HelpMove} }
func (m Model) kbDigits() binding { return binding{key: "1-9", help: m.u().HelpDigits} }
func (m Model) kbEnter() binding  { return binding{key: "Enter", help: m.u().HelpEnter} }
func (m Model) kbSpace() binding  { return binding{key: "Space", help: m.u().HelpSpace} }
func (m Model) kbAll() binding    { return binding{key: "A", help: m.u().HelpAll} }
func (m Model) kbSlash() binding  { return binding{key: "/", help: m.u().HelpSlash} }
func (m Model) kbSearch() binding { return binding{key: "Ctrl+G", help: m.u().HelpSearch} }
func (m Model) kbPickFolder() binding {
	return binding{key: "Ctrl+O", help: m.u().HelpPickFolder}
}
func (m Model) kbEsc() binding { return binding{key: "Esc", help: m.u().HelpBack} }
func (m Model) kbCancel() binding {
	return binding{key: "Esc", help: m.u().HelpCancel}
}
func (m Model) kbAny() binding { return binding{key: m.u().HelpAnyKey, help: m.u().HelpExit} }
func (m Model) kbOpenFolder() binding {
	return binding{key: "O", help: m.u().HelpOpenFolder}
}
func (m Model) menuBindings(extra ...binding) []binding {
	bindings := []binding{m.kbMove(), m.kbDigits(), m.kbEnter()}
	return append(bindings, extra...)
}
func (m Model) playlistBindings() []binding {
	if m.plInputMode {
		return []binding{m.kbEnter(), m.kbEsc()}
	}
	return []binding{m.kbMove(), m.kbSpace(), m.kbEnter(), m.kbAll(), m.kbSlash(), m.kbEsc()}
}
func (m Model) summaryBindings() []binding {
	bindings := []binding{m.kbMove(), m.kbEnter()}
	if m.singleOK || m.dlDone > 0 {
		bindings = append(bindings, m.kbOpenFolder())
	}
	return bindings
}
func (m Model) depBindings() []binding {
	if m.depMode == depModeManage {
		return m.menuBindings(m.kbEsc())
	}
	return m.menuBindings()
}

// ---------- chrome pieces ----------
func (m Model) renderInputWithHint(field inputField, hint string) string {
	parts := []string{renderInputField(field)}
	if hint = strings.TrimSpace(hint); hint != "" {
		parts = append(parts, sInputHint.Render(hint))
	}
	return strings.Join(parts, "\n")
}
func (m Model) renderFooterHelp(bindings ...binding) string {
	parts := make([]string, 0, len(bindings))
	for _, item := range bindings {
		parts = append(parts,
			sHelpBracket.Render("[")+sHelpKey.Render(item.key)+sHelpBracket.Render("]")+
				" "+sHelpText.Render(item.help),
		)
	}
	body := joinFittedParts(m.cardBodyWidth(), parts, "  ·  ")
	return sep(m.cardBodyWidth()) + "\n" + body
}
