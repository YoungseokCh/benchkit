package tui

import (
	"testing"

	benchkit "github.com/YoungseokCh/benchkit"

	tea "charm.land/bubbletea/v2"
)

func TestModelViewDoesNotCaptureMouse(t *testing.T) {
	model := newModel[struct{}](benchkit.SuiteEvent{
		Name:     "copyable",
		Total:    1,
		Parallel: 1,
	}, nil)

	view := model.View()
	if view.MouseMode != tea.MouseModeNone {
		t.Fatalf("MouseMode = %v, want %v", view.MouseMode, tea.MouseModeNone)
	}
}
