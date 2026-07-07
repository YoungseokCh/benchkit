package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestBubbleModelViewDoesNotCaptureMouse(t *testing.T) {
	model := newBubbleModel[struct{}](SuiteEvent{
		Name:     "copyable",
		Total:    1,
		Parallel: 1,
	}, nil)

	view := model.View()
	if view.MouseMode != tea.MouseModeNone {
		t.Fatalf("MouseMode = %v, want %v", view.MouseMode, tea.MouseModeNone)
	}
}
