package tui

import (
	"strconv"
	"strings"
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

func TestModelStartsOnStreamTab(t *testing.T) {
	model := newModel[struct{}](benchkit.SuiteEvent{
		Name:     "stream-first",
		Total:    1,
		Parallel: 1,
	}, nil)

	if model.activeTab != viewTabStream {
		t.Fatalf("activeTab = %v, want %v", model.activeTab, viewTabStream)
	}
}

func TestModelStreamFilterKeepsErrorsVisibleAndCanRevealHiddenDoneCases(t *testing.T) {
	m := newModel[struct{}](benchkit.SuiteEvent{
		Name:     "filtered",
		Total:    2,
		Parallel: 1,
	}, func(benchkit.CaseResult[struct{}]) bool {
		return false
	})

	m.applyCaseFinished(benchkit.WorkerCaseResult[struct{}]{
		Result: benchkit.CaseResult[struct{}]{
			Case:  benchkit.Case{Name: "done-case"},
			State: benchkit.StateDone,
		},
	})
	m.applyCaseFinished(benchkit.WorkerCaseResult[struct{}]{
		Result: benchkit.CaseResult[struct{}]{
			Case:  benchkit.Case{Name: "error-case"},
			State: benchkit.StateError,
			Error: "failed",
		},
	})

	filtered := strings.Join(m.streamLines(), "\n")
	if strings.Contains(filtered, "done-case") {
		t.Fatalf("filtered stream includes hidden done case:\n%s", filtered)
	}
	if !strings.Contains(filtered, "error-case") {
		t.Fatalf("filtered stream omits error case:\n%s", filtered)
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	revealed := strings.Join(updated.(model[struct{}]).streamLines(), "\n")
	if !strings.Contains(revealed, "done-case") {
		t.Fatalf("revealed stream omits hidden done case:\n%s", revealed)
	}
}

func TestModelStreamShowsOnlyRunResults(t *testing.T) {
	m := newModel[struct{}](benchkit.SuiteEvent{
		Name:     "results-only",
		Total:    2,
		Parallel: 1,
	}, nil)

	m.applyCaseStarted(benchkit.WorkerCaseEvent{
		WorkerID: 0,
		Case:     benchkit.Case{Name: "started-case"},
	})
	updated, _ := m.Update(aggregateUpdatedMsg{snapshot: benchkit.Stats{
		{
			Title: "verdict",
			Items: []benchkit.StatItem{{Label: "passed", Value: 1}},
		},
	}})
	m = updated.(model[struct{}])
	m.applyCaseFinished(benchkit.WorkerCaseResult[struct{}]{
		Result: benchkit.CaseResult[struct{}]{
			Case:  benchkit.Case{Name: "done-case"},
			State: benchkit.StateDone,
		},
	})
	updated, _ = m.Update(suiteFinishedMsg[struct{}]{summary: benchkit.Summary[struct{}]{
		Name:  "results-only",
		Total: 2,
		Done:  2,
	}})
	m = updated.(model[struct{}])

	stream := strings.Join(m.streamLines(), "\n")
	for _, hidden := range []string{"Running results-only", "[START]", "Stats:", "Summary:"} {
		if strings.Contains(stream, hidden) {
			t.Fatalf("stream contains lifecycle line %q:\n%s", hidden, stream)
		}
	}
	if !strings.Contains(stream, "done-case") {
		t.Fatalf("stream omits completed result:\n%s", stream)
	}
}

func TestModelStatsTabUsesSharedScrollableViewport(t *testing.T) {
	m := newModel[struct{}](benchkit.SuiteEvent{
		Name:     "scrollable-stats",
		Total:    1,
		Parallel: 1,
	}, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = updated.(model[struct{}])

	rows := make([][]any, 0, 40)
	for i := 0; i < 40; i++ {
		rows = append(rows, []any{"file-" + leftPadForTest(i, 2), 100, i, float64(i) / 100})
	}
	updated, _ = m.Update(aggregateUpdatedMsg{snapshot: benchkit.Stats{
		{
			Title: "coverage",
			Table: &benchkit.StatTable{
				Columns: []string{"file", "total", "covered", "coverage"},
				Rows:    rows,
			},
		},
	}})
	m = updated.(model[struct{}])
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(model[struct{}])

	if m.viewport.YOffset() != 0 {
		t.Fatalf("stats viewport starts at offset %d, want 0", m.viewport.YOffset())
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updated.(model[struct{}])
	if m.viewport.YOffset() == 0 {
		t.Fatal("stats viewport did not scroll on j")
	}
	m.viewport.GotoTop()

	m.viewport.SetHeight(100)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updated.(model[struct{}])
	if m.viewport.YOffset() == 0 {
		t.Fatal("stats viewport did not scroll after stale viewport height")
	}
	m.viewport.GotoTop()

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m = updated.(model[struct{}])
	if m.viewport.YOffset() == 0 {
		t.Fatal("stats viewport did not scroll on page down")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	m = updated.(model[struct{}])
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(model[struct{}])
	if m.viewport.YOffset() != 0 {
		t.Fatalf("stats viewport preserved tab scroll offset %d, want reset to 0", m.viewport.YOffset())
	}
}

func TestModelWithoutStreamFilterShowsCompletedCases(t *testing.T) {
	model := newModel[struct{}](benchkit.SuiteEvent{
		Name:     "unfiltered",
		Total:    1,
		Parallel: 1,
	}, nil)

	model.applyCaseFinished(benchkit.WorkerCaseResult[struct{}]{
		Result: benchkit.CaseResult[struct{}]{
			Case:  benchkit.Case{Name: "done-case"},
			State: benchkit.StateDone,
		},
	})

	stream := strings.Join(model.streamLines(), "\n")
	if !strings.Contains(stream, "done-case") {
		t.Fatalf("stream omits completed case without filter:\n%s", stream)
	}
}

func leftPadForTest(value int, width int) string {
	text := strconv.Itoa(value)
	for len(text) < width {
		text = "0" + text
	}
	return text
}
