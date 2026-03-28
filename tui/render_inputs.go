package tui

import (
	"strings"
)

func (m Model) renderTextInputScreen(title, prompt string, field inputField, errText, hintText string, bindings ...binding) string {
	var b strings.Builder
	b.WriteString(sBold.Render(title) + "\n\n")
	if prompt != "" {
		b.WriteString(sDim.Render(prompt) + "\n")
		b.WriteString("\n")
	}
	b.WriteString(renderInputField(field) + "\n")
	if errText != "" {
		b.WriteString("\n" + renderErrorLine(errText) + "\n")
	} else if hintText != "" {
		b.WriteString(sDim.Render(hintText) + "\n")
	}
	b.WriteString(m.hint(bindings...))
	return b.String()
}

func (m Model) viewURLScreen() string {
	u := m.u()
	return m.renderTextInputScreen(u.PasteURL, "", m.urlInput, m.urlErr, u.URLHints, m.kbEnter(), m.kbSearch(), m.kbQuit(), m.kbUpdDeps())
}

func (m Model) viewSearchInput() string {
	u := m.u()
	return m.renderTextInputScreen(u.SearchTitle, u.SearchPrompt, m.searchInput, m.searchErr, "", m.kbEnter(), m.kbEsc(), m.kbQuit())
}

func (m Model) viewSearchResults() string {
	u := m.u()

	var b strings.Builder
	b.WriteString(sBold.Render(u.SearchTitle) + "\n\n")
	if m.searchQuery != "" {
		b.WriteString(sDim.Render("  "+m.searchQuery) + "\n\n")
	}
	b.WriteString(m.menu.View())
	b.WriteString(m.hint(m.kbUp(), m.kbDown(), m.kbEnter(), m.kbEsc()))
	return b.String()
}

func (m Model) viewFragmentInput() string {
	u := m.u()
	return m.renderTextInputScreen(u.FragmentInputTitle, u.FragmentInputPrompt, m.fragmentIn, m.fragmentErr, u.FragmentInputHint, m.kbEnter(), m.kbEsc(), m.kbQuit())
}
