package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

const (
	barW   = 38
	cardW  = 84
	inputW = 56
	menuW  = 72
)

var (
	cPrimary     = lipgloss.Color("#5BA8F5")
	cPrimarySoft = lipgloss.Color("#A9D2FF")
	cInfo        = lipgloss.Color("#7FB9FF")
	cSuccess     = lipgloss.Color("#4ECCA0")
	cWarn        = lipgloss.Color("#F0B95C")
	cError       = lipgloss.Color("#F08B8B")
	cGray        = lipgloss.Color("#A9BAD4")
	cDim         = lipgloss.Color("#6E82A6")
	cWhite       = lipgloss.Color("#EDF4FF")
	cPanel       = lipgloss.Color("#0E1626")
	cBorder      = lipgloss.Color("#33496E")
	cBorderSoft  = lipgloss.Color("#26395A")
	cAccentDim   = lipgloss.Color("#7C9CC4")

	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(cPrimarySoft)
	sAccent  = lipgloss.NewStyle().Bold(true).Foreground(cAccentDim)
	sOk      = lipgloss.NewStyle().Bold(true).Foreground(cSuccess)
	sErr     = lipgloss.NewStyle().Bold(true).Foreground(cError)
	sWarn    = lipgloss.NewStyle().Bold(true).Foreground(cWarn)
	sBold    = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sCursor  = lipgloss.NewStyle().Bold(true).Foreground(cPanel).Background(cPrimarySoft)
	sLabel   = lipgloss.NewStyle().Foreground(cGray)
	sValue   = lipgloss.NewStyle().Foreground(cWhite)
	sBody    = lipgloss.NewStyle().Foreground(cWhite)
	sBarRest = lipgloss.NewStyle().Foreground(cDim)

	sCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(1, 2)

	sSectionTitle = lipgloss.NewStyle().Bold(true).Foreground(cPrimarySoft)
	sSectionBox   = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(cBorderSoft).
			PaddingLeft(1)
	sRule         = lipgloss.NewStyle().Foreground(cBorder)
	sSubtitle     = lipgloss.NewStyle().Foreground(cGray)
	sMeta         = lipgloss.NewStyle().Foreground(cDim)
	sBadge        = lipgloss.NewStyle()
	sBadgeLabel   = lipgloss.NewStyle().Foreground(cDim)
	sBadgeValue   = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sBadgeHotkey  = lipgloss.NewStyle().Foreground(cPrimarySoft).Bold(true)
	sHelpKey      = lipgloss.NewStyle().Bold(true).Foreground(cPrimarySoft)
	sHelpText     = lipgloss.NewStyle().Foreground(cGray)
	sInputHint    = lipgloss.NewStyle().Foreground(cDim)
	sLink         = lipgloss.NewStyle().Foreground(cPrimarySoft)
	sTableLabel   = lipgloss.NewStyle().Foreground(cGray)
	sTableMeta    = lipgloss.NewStyle().Foreground(cDim)
	sMenuLead     = lipgloss.NewStyle()
	sMenuLeadAct  = sMenuLead.Bold(true).Foreground(cPrimary)
	sMenuIndex    = lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Foreground(cDim)
	sMenuIndexAct = sMenuIndex.Foreground(cPrimary)
	sMenuText     = lipgloss.NewStyle().Foreground(cGray)
	sMenuTextAct  = lipgloss.NewStyle().Bold(true).Foreground(cWhite)

	sNoticeInfo = lipgloss.NewStyle().
			BorderLeft(true).
			BorderForeground(cInfo).
			PaddingLeft(1).
			PaddingRight(1)

	sNoticeSuccess = sNoticeInfo.BorderForeground(cSuccess)
	sNoticeWarn    = sNoticeInfo.BorderForeground(cWarn)
	sNoticeErr     = sNoticeInfo.BorderForeground(cError)
	sNoticeTag     = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginRight(1)

	sInputBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBorderSoft).
			Padding(0, 1)

	sInputBoxFocus = sInputBox.
			BorderForeground(cPrimary)

	sMenuRow = lipgloss.NewStyle().
			Padding(0, 1)

	sMenuActive = sMenuRow.
			Foreground(cWhite)

	sPlTitle   = lipgloss.NewStyle().Inline(true)
	sSlotTitle = lipgloss.NewStyle().Inline(true)

	progressBlendStops = []color.Color{
		lipgloss.Color("#2E67D1"),
		lipgloss.Color("#4F8CFF"),
		lipgloss.Color("#79B5FF"),
	}
)

const (
	progressFullChar  = "█"
	progressEmptyChar = "░"
)
