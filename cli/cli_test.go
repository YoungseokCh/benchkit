package cli

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	benchkit "github.com/YoungseokCh/benchkit"
)

type memoryResultStore[T any] struct {
	summaries map[string]Summary[T]
	loads     []string
	saves     []string
}

func (s *memoryResultStore[T]) Load(resultDir string) (Summary[T], error) {
	s.loads = append(s.loads, resultDir)
	summary, ok := s.summaries[resultDir]
	if !ok {
		return Summary[T]{}, fmt.Errorf("missing summary for %s", resultDir)
	}
	return summary, nil
}

func (s *memoryResultStore[T]) Save(resultDir string, summary Summary[T]) error {
	s.saves = append(s.saves, resultDir)
	if s.summaries == nil {
		s.summaries = make(map[string]Summary[T])
	}
	s.summaries[resultDir] = summary
	return nil
}

func TestCLIUpdatePreservesUnselectedResults(t *testing.T) {
	outputs := map[string]int{"a": 1, "b": 2}
	suite := Benchmark[int]{
		Name: "cli-incremental",
		Cases: []Case{
			{Name: "a"},
			{Name: "b"},
		},
		RunCase: func(_ context.Context, c Case) (benchkit.CaseReport[int], error) {
			return benchkit.CaseReport[int]{Output: outputs[c.Name]}, nil
		},
	}

	resultDir := t.TempDir()
	var out bytes.Buffer
	cli := CLI[int]{Benchmark: suite, Out: &out, Err: &out}
	if err := cli.Run(context.Background(), []string{"-tui=false", "-result-dir", resultDir}); err != nil {
		t.Fatal(err)
	}

	outputs["b"] = 20
	out.Reset()
	if err := cli.Run(context.Background(), []string{"-tui=false", "-result-dir", resultDir, "-update", "-case", "b"}); err != nil {
		t.Fatal(err)
	}

	summary, err := loadSummary[int](resultDir)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || len(summary.Results) != 2 {
		t.Fatalf("expected whole-suite result, got total=%d len=%d", summary.Total, len(summary.Results))
	}
	if summary.Results[0].Case.Name != "a" || summary.Results[0].Output != 1 {
		t.Fatalf("case a was not preserved: %#v", summary.Results[0])
	}
	if summary.Results[1].Case.Name != "b" || summary.Results[1].Output != 20 {
		t.Fatalf("case b was not updated: %#v", summary.Results[1])
	}
}

func TestCLIPassesResultDirAndUsesCustomResultStore(t *testing.T) {
	resultDir := t.TempDir()
	store := &memoryResultStore[int]{}
	aggregateCalls := 0
	suite := Benchmark[int]{
		Name:  "cli-result-dir",
		Cases: []Case{{Name: "a"}},
		Aggregate: func(summary Summary[int]) (any, error) {
			aggregateCalls++
			if summary.ResultDir != resultDir {
				return nil, fmt.Errorf("aggregate saw result dir %q", summary.ResultDir)
			}
			return nil, nil
		},
		RunCase: func(ctx context.Context, _ Case) (benchkit.CaseReport[int], error) {
			if got := benchkit.ResultDir(ctx); got != resultDir {
				return benchkit.CaseReport[int]{}, fmt.Errorf("case saw result dir %q", got)
			}
			return benchkit.CaseReport[int]{Output: 1}, nil
		},
	}

	var out bytes.Buffer
	cli := CLI[int]{Benchmark: suite, ResultStore: store, Out: &out, Err: &out}
	if err := cli.Run(context.Background(), []string{"-tui=false", "-result-dir", resultDir}); err != nil {
		t.Fatal(err)
	}

	if aggregateCalls == 0 {
		t.Fatal("aggregate was not called")
	}
	if len(store.loads) != 0 {
		t.Fatalf("unexpected load calls: %#v", store.loads)
	}
	if len(store.saves) != 1 || store.saves[0] != resultDir {
		t.Fatalf("unexpected save calls: %#v", store.saves)
	}
	summary := store.summaries[resultDir]
	if summary.ResultDir != resultDir {
		t.Fatalf("stored summary saw result dir %q", summary.ResultDir)
	}
}
