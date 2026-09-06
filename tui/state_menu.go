package tui

func (m *Model) syncLocalizedInputs() {
	m.searchInput.SetPlaceholder(m.u().SearchPlaceholder)
	m.plInput.SetPlaceholder(m.u().PlInputPlaceholder)
	m.menu.SetItems(m.menuItems())
	m.syncLayout()
}
