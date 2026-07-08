package tui

import (
	"strings"
	"testing"

	benchkit "github.com/YoungseokCh/benchkit"
)

type streamTestOutput struct {
	Passed    bool `json:"passed"`
	LatencyMS int  `json:"latency_ms"`
}

func TestCaseFinishedStreamLineOmitsDonePrefix(t *testing.T) {
	line := caseFinishedStreamLine(benchkit.WorkerCaseResult[struct{}]{
		WorkerID: 3,
		Result: benchkit.CaseResult[struct{}]{
			Case:     benchkit.Case{Name: "job-076"},
			State:    benchkit.StateDone,
			Duration: 1272,
		},
	})

	if line.Plain != "job-076 (1272ms)" {
		t.Fatalf("Plain = %q", line.Plain)
	}
	if !strings.Contains(line.JSON, `"event":"case_finished"`) {
		t.Fatalf("JSON = %q, want case_finished event", line.JSON)
	}
}

func TestCaseFinishedStreamLineShowsUserOutput(t *testing.T) {
	line := caseFinishedStreamLine(benchkit.WorkerCaseResult[streamTestOutput]{
		WorkerID: 3,
		Result: benchkit.CaseResult[streamTestOutput]{
			Case:     benchkit.Case{Name: "job-076"},
			State:    benchkit.StateDone,
			Duration: 1272,
			Output: streamTestOutput{
				Passed:    false,
				LatencyMS: 1272,
			},
		},
	})

	want := `job-076 {"passed":false,"latency_ms":1272} (1272ms)`
	if line.Plain != want {
		t.Fatalf("Plain = %q, want user output", line.Plain)
	}
}

func TestCaseFinishedStreamLineKeepsErrorPrefix(t *testing.T) {
	line := caseFinishedStreamLine(benchkit.WorkerCaseResult[struct{}]{
		WorkerID: 3,
		Result: benchkit.CaseResult[struct{}]{
			Case:     benchkit.Case{Name: "job-077"},
			State:    benchkit.StateError,
			Error:    "failed",
			Duration: 1272,
		},
	})

	if !strings.HasPrefix(line.Plain, "[ERROR] job-077") {
		t.Fatalf("Plain = %q, want error prefix", line.Plain)
	}
}
