package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func truncate(value string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		for _, r := range value {
			return string(r)
		}
		return ""
	}
	var out strings.Builder
	for _, r := range value {
		next := out.String() + string(r)
		if lipgloss.Width(next)+1 > width {
			break
		}
		out.WriteRune(r)
	}
	return out.String() + "…"
}

func padRight(value string, width int) string {
	for lipgloss.Width(value) < width {
		value += " "
	}
	return value
}

func padLeft(value string, width int) string {
	for lipgloss.Width(value) < width {
		value = " " + value
	}
	return value
}

func leftPad(value string, width int) string {
	for len(value) < width {
		value = "0" + value
	}
	return value
}

// LineCount returns the number of display lines in value.
func LineCount(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}
