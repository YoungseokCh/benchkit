package components

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// Panel renders a fixed-height bordered panel. When Viewport is set, the panel
// includes a scrollbar cell and uses the viewport's visible content as its body.
type Panel struct {
	Title    string
	Tabs     []Tab
	Body     string
	Width    int
	Height   int
	Viewport *viewport.Model
}

// View renders the panel.
func (p Panel) View() string {
	body := p.Body
	if p.Viewport != nil {
		body = p.Viewport.View()
	}
	return panel(p.Title, p.Tabs, body, p.Width, p.Height, p.Viewport)
}

func panel(title string, tabs []Tab, body string, width int, height int, view *viewport.Model) string {
	if width < 20 {
		width = 20
	}
	if height < 3 {
		height = 3
	}
	innerWidth := width - 2
	innerHeight := height - 2
	title = truncate(title, innerWidth)

	top := panelTop(title, tabs, innerWidth)
	contentWidth := innerWidth
	if view != nil {
		contentWidth = innerWidth - 1
	}
	if contentWidth < 1 {
		contentWidth = 1
	}

	bodyLines := strings.Split(body, "\n")
	lines := []string{top}
	for i := 0; i < innerHeight; i++ {
		line := ""
		if i < len(bodyLines) {
			line = bodyLines[i]
		}
		scrollbar := ""
		if view != nil {
			scrollbar = scrollbarCell(*view, i, innerHeight)
		}
		lines = append(
			lines,
			mutedStyle.Render("│")+
				padRight(truncate(line, contentWidth), contentWidth)+
				scrollbar+
				mutedStyle.Render("│"),
		)
	}
	lines = append(lines, mutedStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(lines, "\n")
}

func panelTop(title string, tabs []Tab, innerWidth int) string {
	header := ""
	if len(tabs) > 0 {
		header = RenderTabs(tabs)
	} else if title != "" {
		header = sectionStyle.Render(title)
	}
	fill := innerWidth - lipgloss.Width(header)
	if fill < 0 {
		fill = 0
	}
	return mutedStyle.Render("╭") + header + mutedStyle.Render(strings.Repeat("─", fill)+"╮")
}

// Footer renders the TUI keyboard hint footer.
func Footer(finished bool, hints string) string {
	prefix := hints
	if prefix != "" {
		prefix += "  "
	}
	if finished {
		return mutedStyle.Render(prefix + "scroll arrows/j/k/pageup/pagedown  q/ctrl+c exits after finish")
	}
	return mutedStyle.Render(prefix + "scroll arrows/j/k/pageup/pagedown  q/ctrl+c exits")
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
