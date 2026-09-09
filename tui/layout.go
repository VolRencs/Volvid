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
	return fitWidth(m.width-4, cardW, minCardWidth)
}

func (m Model) cardPadding() (int, int) {
	switch {
	case m.height > 0 && m.height < breakShortHeight:
		return 0, 1
	case m.height > 0 && m.height < breakMediumHeight:
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
	return fitWidth(m.cardBodyWidth(), menuW, minMenuWidth)
}

func (m Model) primaryInputWidth() int {
	return fitWidth(m.cardBodyWidth()-4, inputW, minInputWidth)
}

func (m Model) playlistInputWidth() int {
	return fitWidth(m.cardBodyWidth()-12, 38, minPlaylistInWidth)
}

func (m Model) fragmentInputWidth() int {
	return fitWidth(m.cardBodyWidth()-20, 28, minFragmentInWidth)
}

func (m Model) progressBarWidth() int {
	return fitWidth(m.cardBodyWidth()-18, barW, minBarWidth)
}

func (m Model) playlistTitleWidth() int {
	return fitWidth(m.cardBodyWidth()-18, 40, minTitleWidth)
}

func (m Model) slotTitleWidth() int {
	return fitWidth(m.cardBodyWidth()-14, 46, minSlotTitleWidth)
}

func (m *Model) syncLayout() {
	m.urlInput.SetWidth(m.primaryInputWidth())
	m.searchInput.SetWidth(m.primaryInputWidth())
	m.plInput.SetWidth(m.playlistInputWidth())
	m.fragmentIn.SetWidth(m.fragmentInputWidth())
}

func (m Model) sectionGap() string {
	if m.height > 0 && m.height < breakCompactHeight {
		return "\n"
	}
	return "\n\n"
}

func (m Model) compactHomeLayout() bool {
	return (m.height > 0 && m.height < breakCompactHeight) || (m.width > 0 && m.width < breakNarrowWidth)
}

func (m Model) sectionBodyWidth() int {
	return max(1, m.cardBodyWidth()-4)
}
