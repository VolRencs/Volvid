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
	cPrimary     = lipgloss.Color("#6FB3FF")
	cPrimarySoft = lipgloss.Color("#A9D2FF")
	cPrimaryDeep = lipgloss.Color("#1E3F73")
	cInfo        = lipgloss.Color("#7FB9FF")
	cSuccess     = lipgloss.Color("#57DBA4")
	cWarn        = lipgloss.Color("#F5C06B")
	cError       = lipgloss.Color("#FF9494")
	cGray        = lipgloss.Color("#AFC2DE")
	cDim         = lipgloss.Color("#6E82A6")
	cWhite       = lipgloss.Color("#EFF5FF")
	cPanel       = lipgloss.Color("#0B1220")
	cBorder      = lipgloss.Color("#3A5480")
	cBorderSoft  = lipgloss.Color("#26395A")
	cAccentDim   = lipgloss.Color("#8FB4E3")
)

// Semantic styles.
var (
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
	sBarRest = lipgloss.NewStyle().Foreground(cBorderSoft)
)

// Chrome styles: card, sections, rules.
var (
	sCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(1, 2)

	sSectionTitle = lipgloss.NewStyle().Bold(true).Foreground(cAccentDim)
	sSectionBox   = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(cBorderSoft).
			PaddingLeft(1)
	sRule     = lipgloss.NewStyle().Foreground(cBorderSoft)
	sSubtitle = lipgloss.NewStyle().Foreground(cGray)
	sMeta     = lipgloss.NewStyle().Foreground(cDim)
)

// Badge / chip styles.
var (
	sBadge       = lipgloss.NewStyle()
	sBadgeLabel  = lipgloss.NewStyle().Foreground(cDim)
	sBadgeValue  = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sBadgeHotkey = lipgloss.NewStyle().Foreground(cPrimarySoft).Bold(true)
	sBrandMark   = lipgloss.NewStyle().Bold(true).Foreground(cPrimary)
	sVersionChip = lipgloss.NewStyle().Foreground(cDim)
	sStatusDot   = lipgloss.NewStyle()
	sLocaleChip  = lipgloss.NewStyle().Bold(true).Foreground(cPrimaryDeep).Background(cAccentDim).Padding(0, 1)
)

// Help bar styles: [Key] Label · …
var (
	sHelpBracket = lipgloss.NewStyle().Foreground(cDim)
	sHelpKey     = lipgloss.NewStyle().Bold(true).Foreground(cPrimarySoft)
	sHelpText    = lipgloss.NewStyle().Foreground(cGray)
	sInputHint   = lipgloss.NewStyle().Foreground(cDim)
	sLink        = lipgloss.NewStyle().Foreground(cPrimarySoft)
)

// Table styles.
var (
	sTableLabel = lipgloss.NewStyle().Foreground(cGray)
	sTableMeta  = lipgloss.NewStyle().Foreground(cDim)
)

// List styles: shared by option menus and playlist rows.
var (
	sListLead        = lipgloss.NewStyle()
	sListLeadAct     = sListLead.Bold(true).Foreground(cPrimary)
	sListIndex       = lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Foreground(cDim)
	sListIndexAct    = lipgloss.NewStyle().Width(3).Align(lipgloss.Right).Bold(true).Foreground(cWhite)
	sListItemText    = lipgloss.NewStyle().Foreground(cGray)
	sListItemTextAct = lipgloss.NewStyle()

	sListRow    = lipgloss.NewStyle().Padding(0, 1)
	sListRowAct = lipgloss.NewStyle().
			Bold(true).
			Foreground(cWhite).
			Background(cPrimaryDeep)
)

// Notice styles: left-accent strips with solid tags.
var (
	sNoticeInfo = lipgloss.NewStyle().
			BorderLeft(true).
			BorderForeground(cInfo).
			PaddingLeft(1).
			PaddingRight(1)

	sNoticeSuccess = sNoticeInfo.BorderForeground(cSuccess)
	sNoticeWarn    = sNoticeInfo.BorderForeground(cWarn)
	sNoticeErr     = sNoticeInfo.BorderForeground(cError)
	sNoticeTag     = lipgloss.NewStyle().Bold(true).Foreground(cPanel).Padding(0, 1).MarginRight(1)
)

// Input styles.
var (
	sInputBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBorderSoft).
			Padding(0, 1)

	sInputBoxFocus = sInputBox.
			BorderForeground(cPrimary)
)

// Download slot styles.
var (
	sSlotTitle = lipgloss.NewStyle().Inline(true)
	sSlotBox   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBorderSoft).
			Padding(0, 1)
)

var (
	sPlTitle = lipgloss.NewStyle().Inline(true)
)

var (
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

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	iconCheck  = "✓"
	iconCross  = "✗"
	iconDotOn  = "●"
	iconDotOff = "○"
	iconMarker = "❯"
)
