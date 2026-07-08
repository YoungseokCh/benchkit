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

## Changelog

### Unreleased

- Use one shared scrollable TUI panel for both stats and stream tabs. Switching
  tabs swaps the panel content and resets scroll position instead of preserving
  separate per-tab scroll state.

### 0.1.1

Compared with `v0.1.0`:

- Replaced the pass/fail `Status` model with framework execution `State`
  values: `StateDone`, `StateError`, and `StateSkip`. Domain verdicts such as
  pass/fail now belong in the typed `Output` payload and custom aggregators.
- Replaced `ErrBenchmarkFailed`, `Summary.PassedOK`, `Summary.Passed`, and
  `Summary.Failed` with error-focused `ErrBenchmarkErrored`, `Summary.OK`,
  `Summary.Done`, `Summary.Errors`, and `Summary.Skipped`.
- Removed `CaseReport.Metrics` and `CaseResult.Metrics`; report benchmark data
  through typed `Output` and aggregate it with `Aggregator[T]`.
- Replaced TUI `RecentFilter` / `RecentFailed` with `StreamFilter` /
  `StreamErrors`. Errored cases are always visible, while non-error completed
  cases are filtered by `StreamFilter`.
- Reworked the TUI into a tabbed stats/stream interface with stable worker
  meters, table-rendered stat items, plain/JSON stream modes, and an `a` key to
  reveal completed results hidden by the stream filter.
- Changed the TUI stream to show only completed run results. It no longer mixes
  in start events, aggregate updates, or suite summaries, and plain result lines
  include the user-defined `Output` payload before the duration.
- Removed the CLI `-list` and final-summary `-json` flags; use case filters to
  select runs and `-jsonl` for machine-readable lifecycle output.
- Updated the bundled examples to put domain pass/fail data in `Output` and to
  use custom aggregators for benchmark-specific summaries.

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
		StreamFilter: benchkitcli.StreamErrors[myOutput],
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
tabbed stats/stream panel. The stream can toggle between plain and JSON output.
Redirected output stays plain and
line-oriented so CI logs remain readable.

TUI keys:

- `k` or up arrow: scroll stream output older.
- `j` or down arrow: scroll stream output newer.
- `PageUp` / `PageDown`: scroll by one viewport.
- `g` / `Home`: jump to oldest stream output.
- `G` / `End`: jump to newest stream output.
- `a`: reveal or hide completed cases filtered out of the stream.
- `q` / `Ctrl+C`: exit after the benchmark finishes.
- `?`: toggle help.

## Custom Aggregation

Use a custom `Aggregator[T]` when execution state counts are not enough. Aggregators
see every completed result through `Observe`, can expose live TUI/JSONL state
through `Snapshot`, and return the final `Summary.Aggregated` payload from
`Finalize`.

```go
type coverageFile struct {
	Path       string
	TotalUnits int
}

type fileCoverageOutput struct {
	Path         string `json:"path"`
	TotalUnits   int    `json:"total_units"`
	CoveredUnits []int  `json:"covered_units"`
}

type coverageOutput struct {
	Files []fileCoverageOutput `json:"files"`
}

type fileCoverageAggregate struct {
	totalUnits   int
	covered      []bool
	coveredUnits int
}

type coverageAggregator struct {
	files map[string]*fileCoverageAggregate
	order []string
}

func newCoverageAggregator(files []coverageFile) *coverageAggregator {
	aggregator := &coverageAggregator{files: make(map[string]*fileCoverageAggregate)}
	for _, file := range files {
		aggregator.order = append(aggregator.order, file.Path)
		aggregator.files[file.Path] = &fileCoverageAggregate{
			totalUnits: file.TotalUnits,
			covered:    make([]bool, file.TotalUnits),
		}
	}
	return aggregator
}

func (a *coverageAggregator) Observe(result benchkit.CaseResult[coverageOutput]) error {
	if result.State != benchkit.StateDone {
		return nil
	}
	for _, file := range result.Output.Files {
		aggregate := a.files[file.Path]
		for _, unit := range file.CoveredUnits {
			if unit >= 0 && unit < aggregate.totalUnits && !aggregate.covered[unit] {
				aggregate.covered[unit] = true
				aggregate.coveredUnits++
			}
		}
	}
	return nil
}

func (a *coverageAggregator) Snapshot() any {
	totalUnits := 0
	coveredUnits := 0
	rows := make([][]any, 0, len(a.order))
	for _, path := range a.order {
		file := a.files[path]
		totalUnits += file.totalUnits
		coveredUnits += file.coveredUnits
		rows = append(rows, []any{
			path,
			file.totalUnits,
			file.coveredUnits,
			float64(file.coveredUnits) / float64(file.totalUnits),
		})
	}
	coverage := 0.0
	if totalUnits > 0 {
		coverage = float64(coveredUnits) / float64(totalUnits)
	}
	return benchkit.Stats{
		{
			Title: "coverage",
			Items: []benchkit.StatItem{
				{Label: "files", Value: len(a.order)},
				{Label: "total_units", Value: totalUnits},
				{Label: "covered_units", Value: coveredUnits},
				{Label: "coverage", Value: coverage},
			},
			Table: &benchkit.StatTable{
				Columns: []string{"file", "total", "covered", "coverage"},
				Rows:    rows,
			},
		},
	}
}

func (a *coverageAggregator) Finalize(summary benchkit.Summary[coverageOutput]) (any, error) {
	return a.Snapshot(), nil
}
```

`benchkit.Stats` is optional, but returning it gives the terminal output a
structured way to render labeled stat items and tables. Aggregators can return
any JSON-marshalable value.

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
