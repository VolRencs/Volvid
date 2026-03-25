package tui

import (
	"strings"
)

func renderInputField(field inputField) string {
	style := sInputBox
	if field.Focused() {
		style = sInputBoxFocus
	}
	return style.Render(field.View())
}

func renderErrorLine(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return sErr.Render("  ✘  " + text)
}
