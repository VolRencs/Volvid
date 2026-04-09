package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

const (
	barW   = 38
	cardW  = 82
	inputW = 56
	menuW  = 68
)

var (
	cPrimary      = lipgloss.Color("#67A8FF")
	cPrimarySoft  = lipgloss.Color("#9CCEFF")
	cPrimaryMuted = lipgloss.Color("#2C5EAA")
	cInfo         = lipgloss.Color("#7FB9FF")
	cSuccess      = lipgloss.Color("#86D6C6")
	cWarn         = lipgloss.Color("#F1C878")
	cError        = lipgloss.Color("#F0A0A0")
	cGray         = lipgloss.Color("#A7B8D2")
	cDim          = lipgloss.Color("#7284A1")
	cWhite        = lipgloss.Color("#F4F8FF")
	cBorder       = lipgloss.Color("#30496E")
	cBorderSoft   = lipgloss.Color("#223551")
	cPanel        = lipgloss.Color("#101A2B")

	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(cPrimarySoft)
	sOk      = lipgloss.NewStyle().Bold(true).Foreground(cSuccess)
	sErr     = lipgloss.NewStyle().Bold(true).Foreground(cError)
	sWarn    = lipgloss.NewStyle().Bold(true).Foreground(cWarn)
	sGray    = lipgloss.NewStyle().Foreground(cGray)
	sBold    = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sNormal  = sBold.Bold(false)
	sCursor  = lipgloss.NewStyle().Bold(true).Foreground(cPanel).Background(cPrimarySoft)
	sLabel   = lipgloss.NewStyle().Foreground(cGray)
	sValue   = lipgloss.NewStyle().Foreground(cWhite)
	sBody    = lipgloss.NewStyle().Foreground(cWhite)
	sBarRest = lipgloss.NewStyle().Foreground(cDim)

	sCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(1, 2)

	sSectionTitle = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sSectionBox   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBorderSoft).
			Padding(0, 1)
	sRule         = lipgloss.NewStyle().Foreground(cBorderSoft)
	sSubtitle     = lipgloss.NewStyle().Foreground(cGray)
	sMeta         = lipgloss.NewStyle().Foreground(cDim)
	sBadge        = lipgloss.NewStyle()
	sBadgeLabel   = lipgloss.NewStyle().Foreground(cDim)
	sBadgeValue   = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sBadgeHotkey  = lipgloss.NewStyle().Foreground(cPrimarySoft).Bold(true)
	sHelpKey      = lipgloss.NewStyle().Foreground(cPrimarySoft).Bold(true)
	sHelpText     = lipgloss.NewStyle().Foreground(cGray)
	sInputHint    = lipgloss.NewStyle().Foreground(cGray)
	sLink         = lipgloss.NewStyle().Foreground(cPrimarySoft)
	sTableLabel   = lipgloss.NewStyle().Foreground(cGray)
	sTableMeta    = lipgloss.NewStyle().Foreground(cDim)
	sMenuLead     = lipgloss.NewStyle()
	sMenuLeadAct  = sMenuLead.Copy().Bold(true).Foreground(cPrimarySoft)
	sMenuIndex    = lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Foreground(cDim)
	sMenuIndexAct = sMenuIndex.Copy().Foreground(cPrimarySoft)
	sMenuText     = lipgloss.NewStyle().Foreground(cGray)
	sMenuTextAct  = lipgloss.NewStyle().Bold(true).Foreground(cWhite)

	sNoticeInfo = lipgloss.NewStyle().
			BorderLeft(true).
			BorderForeground(cInfo).
			PaddingLeft(1).
			PaddingRight(1)

	sNoticeSuccess = sNoticeInfo.Copy().BorderForeground(cSuccess)
	sNoticeWarn    = sNoticeInfo.Copy().BorderForeground(cWarn)
	sNoticeErr     = sNoticeInfo.Copy().BorderForeground(cError)
	sNoticeTag     = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginRight(1)

	sInputBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBorderSoft).
			Padding(0, 1)

	sInputBoxFocus = sInputBox.Copy().
			BorderForeground(cPrimary)

	sMenuRow = lipgloss.NewStyle().
			Padding(0, 1)

	sMenuActive = sMenuRow.Copy().
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
	progressFullChar  = "▌"
	progressEmptyChar = "░"
)
