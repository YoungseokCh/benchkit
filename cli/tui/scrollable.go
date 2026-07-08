package tui

import (
	"github.com/YoungseokCh/benchkit/cli/tui/components"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type scrollablePanel struct {
	viewport viewport.Model
}

func newScrollablePanel() scrollablePanel {
	panel := scrollablePanel{viewport: viewport.New()}
	panel.viewport.MouseWheelDelta = 1
	return panel
}

func (p *scrollablePanel) configure(width int, height int) {
	viewportWidth, viewportHeight := components.PanelViewportSize(width, height)
	p.viewport.SetWidth(viewportWidth)
	p.viewport.SetHeight(viewportHeight)
}

func (p *scrollablePanel) setContent(content string, preserveYOffset bool) {
	yOffset := p.viewport.YOffset()
	p.viewport.SetContent(content)
	if preserveYOffset {
		p.viewport.SetYOffset(yOffset)
	}
}

func (p *scrollablePanel) gotoTop() {
	p.viewport.GotoTop()
}

func (p *scrollablePanel) gotoBottom() {
	p.viewport.GotoBottom()
}

func (p *scrollablePanel) atBottom() bool {
	return p.viewport.AtBottom()
}

func (p *scrollablePanel) yOffset() int {
	return p.viewport.YOffset()
}

func (p *scrollablePanel) setHeight(height int) {
	p.viewport.SetHeight(height)
}

func (p *scrollablePanel) view(title string, tabs []components.Tab, width int, height int) string {
	p.configure(width, height)
	return components.Panel{
		Title:    title,
		Tabs:     tabs,
		Width:    width,
		Height:   height,
		Viewport: &p.viewport,
	}.View()
}

func (p *scrollablePanel) update(msg tea.Msg) (scrollablePanel, tea.Cmd) {
	viewport, cmd := p.viewport.Update(msg)
	p.viewport = viewport
	return *p, cmd
}

func (p *scrollablePanel) handleKey(msg tea.KeyPressMsg) bool {
	switch {
	case keyPressMatches(msg, "down", "j"):
		p.viewport.ScrollDown(1)
	case keyPressMatches(msg, "up", "k"):
		p.viewport.ScrollUp(1)
	case keyPressMatches(msg, "pgdown", "pagedown", "space", "f"):
		p.viewport.PageDown()
	case keyPressMatches(msg, "pgup", "pageup", "b"):
		p.viewport.PageUp()
	case keyPressMatches(msg, "d", "ctrl+d"):
		p.viewport.HalfPageDown()
	case keyPressMatches(msg, "u", "ctrl+u"):
		p.viewport.HalfPageUp()
	case keyPressMatches(msg, "home", "g"):
		p.viewport.GotoTop()
	case keyPressMatches(msg, "end", "G"):
		p.viewport.GotoBottom()
	default:
		switch msg.Key().Code {
		case tea.KeyDown:
			p.viewport.ScrollDown(1)
		case tea.KeyUp:
			p.viewport.ScrollUp(1)
		case tea.KeyPgDown:
			p.viewport.PageDown()
		case tea.KeyPgUp:
			p.viewport.PageUp()
		case tea.KeyHome:
			p.viewport.GotoTop()
		case tea.KeyEnd:
			p.viewport.GotoBottom()
		default:
			return false
		}
	}
	return true
}

func (p *scrollablePanel) handleMouse(msg tea.MouseWheelMsg) bool {
	switch msg.Mouse().Button {
	case tea.MouseWheelUp:
		p.viewport.ScrollUp(p.viewport.MouseWheelDelta)
	case tea.MouseWheelDown:
		p.viewport.ScrollDown(p.viewport.MouseWheelDelta)
	default:
		return false
	}
	return true
}
