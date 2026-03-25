package tui

import (
	"strings"
)

func (m Model) viewURLScreen() string {
	u := m.u()

	var b strings.Builder
	b.WriteString(sBold.Render(u.PasteURL) + "\n\n")
	b.WriteString(renderInputField(m.urlInput) + "\n")

	if m.urlErr != "" {
		b.WriteString("\n" + renderErrorLine(m.urlErr) + "\n")
	} else {
		b.WriteString(sDim.Render(u.URLHints) + "\n")
	}
	b.WriteString(m.hint(m.kbEnter(), m.kbSearch(), m.kbQuit(), m.kbUpdDeps()))
	return b.String()
}

func (m Model) viewSearchInput() string {
	u := m.u()

	var b strings.Builder
	b.WriteString(sBold.Render(u.SearchTitle) + "\n\n")
	b.WriteString(sDim.Render(u.SearchPrompt) + "\n")
	b.WriteString("\n" + renderInputField(m.searchInput) + "\n")
	if m.searchErr != "" {
		b.WriteString("\n" + renderErrorLine(m.searchErr) + "\n")
	}
	b.WriteString(m.hint(m.kbEnter(), m.kbEsc(), m.kbQuit()))
	return b.String()
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
