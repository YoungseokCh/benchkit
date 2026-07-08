package components

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// Block renders a bordered text block.
func Block(title string, body string, width int) string {
	if body == "" {
		return ""
	}
	if width < 20 {
		width = 20
	}
	innerWidth := width - 2
	title = truncate(title, innerWidth)
	topFill := innerWidth - lipgloss.Width(title)
	if topFill < 0 {
		topFill = 0
	}

	top := mutedStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	if title != "" {
		top = mutedStyle.Render("╭") + sectionStyle.Render(title) + mutedStyle.Render(strings.Repeat("─", topFill)+"╮")
	}
	lines := []string{top}
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, mutedStyle.Render("│")+padRight(truncate(line, innerWidth), innerWidth)+mutedStyle.Render("│"))
	}
	lines = append(lines, mutedStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(lines, "\n")
}

// ViewportBlock renders the recent results viewport with a scrollbar.
func ViewportBlock(body string, view viewport.Model, width int, height int) string {
	if width < 20 {
		width = 20
	}
	if height < 1 {
		height = 1
	}
	innerWidth := width - 2
	contentWidth := innerWidth - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	bodyLines := strings.Split(body, "\n")

	lines := []string{mutedStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")}
	for i := 0; i < height; i++ {
		line := ""
		if i < len(bodyLines) {
			line = bodyLines[i]
		}
		lines = append(
			lines,
			mutedStyle.Render("│")+
				padRight(truncate(line, contentWidth), contentWidth)+
				scrollbarCell(view, i, height)+
				mutedStyle.Render("│"),
		)
	}
	lines = append(lines, mutedStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(lines, "\n")
}

// Footer renders the TUI keyboard hint footer.
func Footer(finished bool) string {
	if finished {
		return mutedStyle.Render("scroll arrows/j/k/pageup/pagedown  q/ctrl+c exits after finish")
	}
	return mutedStyle.Render("scroll arrows/j/k/pageup/pagedown  q/ctrl+c exits")
}

func scrollbarCell(view viewport.Model, row int, height int) string {
	total := view.TotalLineCount()
	visible := view.VisibleLineCount()
	if height <= 0 || total <= visible || visible <= 0 {
		return mutedStyle.Render(" ")
	}

	thumbHeight := visible * height / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	if thumbHeight > height {
		thumbHeight = height
	}

	maxTop := height - thumbHeight
	thumbTop := 0
	if maxTop > 0 {
		thumbTop = int(view.ScrollPercent()*float64(maxTop) + 0.5)
	}
	if row >= thumbTop && row < thumbTop+thumbHeight {
		return meterDoneStyle.Render("█")
	}
	return mutedStyle.Render("│")
}
