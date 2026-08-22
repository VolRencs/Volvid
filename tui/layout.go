package tui

func fitWidth(available, preferred, minWidth int) int {
	if available <= 0 {
		return 1
	}
	if available >= preferred {
		return preferred
	}
	if available >= minWidth {
		return available
	}
	return max(available, 1)
}

func (m Model) cardWidth() int {
	if m.width <= 0 {
		return cardW
	}
	return fitWidth(m.width-4, cardW, 38)
}

func (m Model) cardPadding() (int, int) {
	switch {
	case m.height > 0 && m.height < 28:
		return 0, 1
	case m.height > 0 && m.height < 34:
		return 0, 2
	default:
		return 1, 2
	}
}

func (m Model) cardBodyWidth() int {
	_, px := m.cardPadding()
	if m.width <= 0 {
		return max(1, cardW-(px*2)-2)
	}
	return max(1, m.cardWidth()-(px*2)-2)
}

func (m Model) menuWidth() int {
	return fitWidth(m.cardBodyWidth(), menuW, 24)
}

func (m Model) primaryInputWidth() int {
	return fitWidth(m.cardBodyWidth()-4, inputW, 18)
}

func (m Model) playlistInputWidth() int {
	return fitWidth(m.cardBodyWidth()-12, 38, 14)
}

func (m Model) fragmentInputWidth() int {
	return fitWidth(m.cardBodyWidth()-20, 28, 12)
}

func (m Model) progressBarWidth() int {
	return fitWidth(m.cardBodyWidth()-18, barW, 12)
}

func (m Model) playlistTitleWidth() int {
	return fitWidth(m.cardBodyWidth()-18, 40, 16)
}

func (m Model) slotTitleWidth() int {
	return fitWidth(m.cardBodyWidth()-14, 46, 18)
}

func (m *Model) syncLayout() {
	m.urlInput.SetWidth(m.primaryInputWidth())
	m.searchInput.SetWidth(m.primaryInputWidth())
	m.plInput.SetWidth(m.playlistInputWidth())
	m.fragmentIn.SetWidth(m.fragmentInputWidth())
}

func (m Model) sectionGap() string {
	if m.height > 0 && m.height < 31 {
		return "\n"
	}
	return "\n\n"
}

func (m Model) compactHomeLayout() bool {
	return (m.height > 0 && m.height < 31) || (m.width > 0 && m.width < 78)
}

func (m Model) sectionBodyWidth() int {
	return max(1, m.cardBodyWidth()-4)
}
