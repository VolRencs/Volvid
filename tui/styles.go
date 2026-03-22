package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

const (
	barW   = 40
	inputW = 50
	menuW  = 60
)

var (
	cPrimary = lipgloss.Color("12")
	cRed     = lipgloss.Color("9")
	cYellow  = lipgloss.Color("11")
	cGray    = lipgloss.Color("8")
	cDim     = lipgloss.Color("240")
	cWhite   = lipgloss.Color("15")

	sTitle  = lipgloss.NewStyle().Bold(true).Foreground(cPrimary)
	sOk     = lipgloss.NewStyle().Foreground(cPrimary)
	sErr    = lipgloss.NewStyle().Foreground(cRed)
	sWarn   = lipgloss.NewStyle().Foreground(cYellow)
	sGray   = lipgloss.NewStyle().Foreground(cGray)
	sBold   = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sNormal = sBold.Bold(false)
	sCursor = lipgloss.NewStyle().Reverse(true).Bold(true)

	sHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(cPrimary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cPrimary).
		Padding(0, 3)

	sInputBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cGray).
			Padding(0, 1)

	sInputBoxFocus = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cPrimary).
			Padding(0, 1)

	sPlTitle   = lipgloss.NewStyle().Width(44).Inline(true)
	sSlotTitle = lipgloss.NewStyle().Width(46).Inline(true)
	sBarRest   = lipgloss.NewStyle().Foreground(cDim)

	progressBlendStops = []color.Color{
		lipgloss.Color("#2F66FF"),
		lipgloss.Color("#3F8BFF"),
		lipgloss.Color("#58B8FF"),
	}
)

const (
	progressFullChar  = "▌"
	progressEmptyChar = "░"
)
