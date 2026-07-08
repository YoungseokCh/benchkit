# benchkit

`benchkit` is a Go benchmark harness for arbitrary benchmark suites. You define
the cases and the per-case runner; benchkit handles filtering, bounded
parallelism, per-case timing, aggregation, terminal output, and JSON/JSONL
reporting.

## Install

Install the module:

```sh
go get github.com/YoungseokCh/benchkit
```

Then import the core package and, when building a command-line benchmark, the
CLI helper package:

```go
import (
	benchkit "github.com/YoungseokCh/benchkit"
	benchkitcli "github.com/YoungseokCh/benchkit/cli"
)
```

## Core Concepts

- `Case`: one benchmark input. Put lightweight string metadata in `Meta`, or
  close over richer data from your runner.
- `Runner[T]`: executes one `Case` and returns a typed `CaseReport[T]`.
- `CaseReport[T]`: carries your typed output, optional execution state, and
  message.
- `Aggregator[T]`: observes completed results and returns any JSON-marshalable
  final summary.
- `CLI[T]`: exposes filtering, interactive selection, TUI progress, JSON, and
  JSONL output for your suite.

`StateDone`, `StateError`, and `StateSkip` describe whether a case executed,
errored, or was skipped. Domain verdicts such as pass/fail belong in your typed
`Output`, where custom aggregators can summarize them.

## Minimal Library Usage

```go
package main

import (
	"context"
	"fmt"
	"strconv"

	benchkit "github.com/YoungseokCh/benchkit"
)

type result struct {
	Input  int  `json:"input"`
	Value  int  `json:"value"`
	Passed bool `json:"passed"`
}

func main() {
	suite := benchkit.Benchmark[result]{
		Name: "example",
		Cases: []benchkit.Case{
			{Name: "small", Tags: []string{"smoke"}, Meta: map[string]string{"n": "10"}},
			{Name: "large", Tags: []string{"full"}, Meta: map[string]string{"n": "100"}},
		},
		RunCase: func(ctx context.Context, c benchkit.Case) (benchkit.CaseReport[result], error) {
			n, err := strconv.Atoi(c.Meta["n"])
			if err != nil {
				return benchkit.CaseReport[result]{}, err
			}

			value := n * 2
			report := benchkit.CaseReport[result]{
				Output: result{
					Input:  n,
					Value:  value,
					Passed: value <= 200,
				},
			}
			return report, nil
		},
		Aggregator: &benchkit.SummaryAggregator[result]{},
	}

	summary, err := suite.Run(context.Background(), benchkit.RunOptions[result]{
		Parallel: 4,
		Tags:     []string{"smoke"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s: %d done, %d errors\n", summary.Name, summary.Done, summary.Errors)
}
```

## Build a Benchmark CLI

Most benchmark programs should wrap the suite with `benchkitcli.CLI[T]`:

```go
package main

import (
	"context"
	"os"

	benchkit "github.com/YoungseokCh/benchkit"
	benchkitcli "github.com/YoungseokCh/benchkit/cli"
)

func main() {
	suite := benchkit.Benchmark[myOutput]{
		Name:       "my-benchmark",
		Cases:      makeCases(),
		RunCase:    runCase,
		Aggregator: &benchkit.SummaryAggregator[myOutput]{},
	}

	err := benchkitcli.CLI[myOutput]{
		Benchmark:    suite,
		RecentFilter: benchkitcli.RecentErrors[myOutput],
	}.Run(context.Background(), os.Args[1:])
	os.Exit(benchkitcli.ExitCode(err))
}
```

The CLI supports:

- `-parallel N`: run up to `N` cases concurrently.
- `-case a,b`: run exact case names.
- `-tag smoke,linux`: require all listed tags.
- `-match text`: substring match on case name.
- `-interactive`: prompt for case selection.
- `-tui=false`: disable the terminal UI and print plain progress lines.
- `-jsonl`: stream lifecycle events as JSON lines.

The default terminal output uses a Bubble Tea TUI with whole-terminal progress,
suite ETA, execution state counts, stable worker meters, aggregate snapshots, and a
scrollable recent-results viewport. Redirected output stays plain and
line-oriented so CI logs remain readable.

TUI keys:

- `k` or up arrow: scroll recent results older.
- `j` or down arrow: scroll recent results newer.
- `PageUp` / `PageDown`: scroll by one viewport.
- `g` / `Home`: jump to oldest recent result.
- `G` / `End`: jump to newest recent result.
- `q` / `Ctrl+C`: exit after the benchmark finishes.
- `?`: toggle help.

## Custom Aggregation

Use a custom `Aggregator[T]` when execution state counts are not enough. Aggregators
see every completed result through `Observe`, can expose live TUI/JSONL state
through `Snapshot`, and return the final `Summary.Aggregated` payload from
`Finalize`.

```go
type coverageOutput struct {
	TotalUnits   int `json:"total_units"`
	CoveredUnits int `json:"covered_units"`
}

type coverageAggregator struct {
	totalUnits   int
	coveredUnits int
}

func (a *coverageAggregator) Observe(result benchkit.CaseResult[coverageOutput]) error {
	a.totalUnits += result.Output.TotalUnits
	a.coveredUnits += result.Output.CoveredUnits
	return nil
}

func (a *coverageAggregator) Snapshot() any {
	coverage := 0.0
	if a.totalUnits > 0 {
		coverage = float64(a.coveredUnits) / float64(a.totalUnits)
	}
	return benchkit.Stats{
		{
			Title: "coverage",
			Items: []benchkit.StatItem{
				{Label: "total_units", Value: a.totalUnits},
				{Label: "covered_units", Value: a.coveredUnits},
				{Label: "coverage", Value: coverage},
			},
		},
	}
}

func (a *coverageAggregator) Finalize(summary benchkit.Summary[coverageOutput]) (any, error) {
	return a.Snapshot(), nil
}
```

`benchkit.Stats` is optional, but returning it gives the terminal output a
structured way to render compact key/value sections and tables. Aggregators can
return any JSON-marshalable value.

## Examples

Run the bundled testing example. It generates 120 synthetic jobs with mixed
durations and pass/fail oracle results. Its custom aggregator counts
`Output.Passed` values in the stat view:

```sh
go run ./example/testing -parallel 2
go run ./example/testing -parallel 16
go run ./example/testing -jsonl
go run ./example/testing -interactive
go run ./example/testing -tag smoke
go run ./example/testing -match job-04
```

Run the coverage example. It uses a custom
aggregator to compute benchmark-specific coverage:

```sh
go run ./example/coverage -parallel 16
go run ./example/coverage -tag low
```
