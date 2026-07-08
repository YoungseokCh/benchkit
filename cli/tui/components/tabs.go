package components

import "strings"

// Tab is one selectable TUI view tab.
type Tab struct {
	Label  string
	Active bool
}

// RenderTabs renders compact htop-style tabs for use inside a panel border.
func RenderTabs(tabs []Tab) string {
	var rendered []string
	for i, tab := range tabs {
		label := tab.Label
		if i < 9 {
			label = string(rune('1'+i)) + " " + label
		}
		cell := " " + label + " "
		if tab.Active {
			rendered = append(rendered, activeTabStyle.Render(cell))
		} else {
			rendered = append(rendered, inactiveTabStyle.Render(cell))
		}
	}
	return strings.Join(rendered, "")
}
