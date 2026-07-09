package benchkit

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestRunIncrementalReplacesResultsBeforeAggregating(t *testing.T) {
	bench := Benchmark[int]{
		Name: "incremental",
		Cases: []Case{
			{Name: "a"},
			{Name: "b"},
		},
		Aggregate: func(summary Summary[int]) (any, error) {
			aggregate := map[string]int{}
			for _, result := range summary.Results {
				switch result.State {
				case StateDone, "":
					aggregate["done"]++
					aggregate["sum"] += result.Output
				case StateError:
					aggregate["errors"]++
				}
			}
			return aggregate, nil
		},
		RunCase: func(context.Context, Case) (CaseReport[int], error) {
			return CaseReport[int]{Output: 20}, nil
		},
	}

	previous := Summary[int]{
		Name:   "incremental",
		Total:  2,
		Done:   1,
		Errors: 1,
		Results: []CaseResult[int]{
			{Case: Case{Name: "a"}, Output: 1, State: StateDone},
			{Case: Case{Name: "b"}, Output: 2, State: StateError},
		},
	}

	summary, err := bench.RunIncremental(context.Background(), RunOptions[int]{
		Names: []string{"b"},
	}, previous)
	if err != nil {
		t.Fatal(err)
	}

	if summary.Total != 2 || summary.Done != 2 || summary.Errors != 0 {
		t.Fatalf("unexpected counts: total=%d done=%d errors=%d", summary.Total, summary.Done, summary.Errors)
	}
	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 merged results, got %d", len(summary.Results))
	}
	if summary.Results[1].Case.Name != "b" || summary.Results[1].Output != 20 || summary.Results[1].State != StateDone {
		t.Fatalf("case b was not replaced: %#v", summary.Results[1])
	}

	aggregate, ok := summary.Aggregated.(map[string]int)
	if !ok {
		t.Fatalf("unexpected aggregate type %T", summary.Aggregated)
	}
	if aggregate["done"] != 2 || aggregate["errors"] != 0 || aggregate["sum"] != 21 {
		t.Fatalf("unexpected aggregate: %#v", aggregate)
	}
}

func TestRunOptionsResultDirIsVisibleToCasesAndAggregates(t *testing.T) {
	resultDir := t.TempDir()
	aggregateCalls := 0
	bench := Benchmark[int]{
		Name:  "result-dir",
		Cases: []Case{{Name: "a"}},
		Aggregate: func(summary Summary[int]) (any, error) {
			aggregateCalls++
			if summary.ResultDir != resultDir {
				return nil, fmt.Errorf("aggregate saw result dir %q", summary.ResultDir)
			}
			return nil, nil
		},
		RunCase: func(ctx context.Context, _ Case) (CaseReport[int], error) {
			if got := ResultDir(ctx); got != resultDir {
				return CaseReport[int]{}, fmt.Errorf("case saw result dir %q", got)
			}
			return CaseReport[int]{Output: 1}, nil
		},
	}

	summary, err := bench.Run(context.Background(), RunOptions[int]{ResultDir: resultDir})
	if err != nil {
		t.Fatal(err)
	}
	if summary.ResultDir != resultDir {
		t.Fatalf("summary saw result dir %q", summary.ResultDir)
	}
	if aggregateCalls == 0 {
		t.Fatal("aggregate was not called")
	}
}

func TestValidateRejectsDuplicateCaseNames(t *testing.T) {
	bench := Benchmark[int]{
		Name:  "duplicates",
		Cases: []Case{{Name: "a"}, {Name: "a"}},
		RunCase: func(context.Context, Case) (CaseReport[int], error) {
			return CaseReport[int]{}, nil
		},
	}

	_, err := bench.Run(context.Background(), RunOptions[int]{})
	if err == nil {
		t.Fatal("expected duplicate case error")
	}
	if !strings.Contains(err.Error(), "duplicate case name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunIncrementalRejectsPreviousSuiteMismatch(t *testing.T) {
	bench := Benchmark[int]{
		Name:  "new",
		Cases: []Case{{Name: "a"}},
		RunCase: func(context.Context, Case) (CaseReport[int], error) {
			return CaseReport[int]{}, nil
		},
	}

	_, err := bench.RunIncremental(context.Background(), RunOptions[int]{}, Summary[int]{Name: "old"})
	if err == nil {
		t.Fatal("expected suite mismatch error")
	}
	if !strings.Contains(err.Error(), "not") {
		t.Fatalf("unexpected error: %v", err)
	}
}
