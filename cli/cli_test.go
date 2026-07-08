package cli

import (
	"bytes"
	"context"
	"testing"

	benchkit "github.com/YoungseokCh/benchkit"
)

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
