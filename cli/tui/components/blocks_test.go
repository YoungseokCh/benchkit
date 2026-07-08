package components

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
)

func TestPanelSizesViewportBeforeRenderingScrollbar(t *testing.T) {
	view := viewport.New()
	view.SetWidth(80)
	view.SetHeight(100)
	view.SetContent(strings.Join([]string{
		"line 01",
		"line 02",
		"line 03",
		"line 04",
		"line 05",
		"line 06",
		"line 07",
		"line 08",
		"line 09",
		"line 10",
	}, "\n"))

	rendered := Panel{
		Title:    "scrollable",
		Width:    30,
		Height:   6,
		Viewport: &view,
	}.View()

	if view.Height() != 4 {
		t.Fatalf("viewport height = %d, want 4", view.Height())
	}
	if !strings.Contains(rendered, "█") {
		t.Fatalf("rendered panel does not show scrollbar thumb:\n%s", rendered)
	}
}

func TestPanelRendersScrollbarTrackWhenViewportFits(t *testing.T) {
	view := viewport.New()
	view.SetContent("one\nline")

	rendered := Panel{
		Title:    "fitting",
		Width:    30,
		Height:   6,
		Viewport: &view,
	}.View()

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 || strings.Count(lines[1], "│") < 3 {
		t.Fatalf("rendered panel does not show scrollbar track:\n%s", rendered)
	}
	if strings.Contains(rendered, "█") {
		t.Fatalf("rendered panel shows scrollbar thumb for fitting content:\n%s", rendered)
	}
}
