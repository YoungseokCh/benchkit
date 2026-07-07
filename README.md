# benchkit

`benchkit` is a Go benchmark harness for arbitrary benchmark run.

Users provide:

- `Runner[T]`: executes one arbitrary benchmark case and returns a `CaseReport[T]`.
- `Aggregator[T]`: observes results and returns any final summary object.
- `CLI[T]`: adds interactive selection plus JSON or JSONL machine output.

`PASS`, `FAIL`, `ERROR`, and `SKIP` are framework presets for default counting
and exit-code behavior. They are optional. Domain-specific results such as
coverage, precision, recall, cost, or latency distributions can leave status
empty and report through `CaseReport.Metrics` plus a custom `Aggregator[T]`.
Aggregators also expose `Snapshot()` so the TUI and JSONL output can show live
aggregate values while cases are still running.

```go
import "benchkit"

suite := benchkit.Benchmark[int]{
	Name: "example",
	Cases: []benchkit.Case{
		{Name: "small", Tags: []string{"smoke"}, Meta: map[string]string{"n": "10"}},
	},
	RunCase: func(ctx context.Context, c benchkit.Case) (benchkit.CaseReport[int], error) {
		got := 10
		if got == 10 {
			return benchkit.CaseReport[int]{Output: got, Status: benchkit.StatusPass}, nil
		}
		return benchkit.CaseReport[int]{
			Output: got,
			Status: benchkit.StatusFail,
			Message: "unexpected result",
		}, nil
	},
	Aggregator: &benchkit.SummaryAggregator[int]{},
}

summary, err := suite.Run(context.Background(), benchkit.RunOptions[int]{Parallel: 4})
```

Run the bundled testing example. It generates 120 synthetic jobs with mixed
durations and pass/fail oracle results:

```sh
go run ./example/testing -parallel 2
go run ./example/testing -parallel 16
go run ./example/testing -json
go run ./example/testing -jsonl
go run ./example/testing -interactive
go run ./example/testing -tag smoke
go run ./example/testing -match job-04
```

Run the coverage example. It leaves case status empty and uses a custom
aggregator to compute benchmark-specific coverage:

```sh
go run ./example/coverage -parallel 16
go run ./example/coverage -json
go run ./example/coverage -tag low
```

CLI flags supported by `benchkitcli.CLI[T]`:

- `-interactive`: prompt for case selection.
- `-parallel N`: run up to `N` cases concurrently.
- `-tui=false`: disable the terminal UI and print plain progress lines.
- `-case a,b`: run exact case names.
- `-tag smoke,linux`: require all listed tags.
- `-match text`: substring match on case name.
- `-list`: list selected cases without running.
- `-json`: write final summary as JSON.
- `-jsonl`: stream lifecycle events as JSON lines.

Default terminal output uses a Bubble Tea TUI with the whole terminal screen:
progress, suite ETA, pass/fail counts, htop-style stable worker meters, and a
scrollable recent results viewport that fills the remaining screen height.
Worker meter progress is estimated from the average completed case duration.
Redirected output stays plain line-oriented so CI logs remain readable.

TUI keys:

- `k` or up arrow: scroll recent results older.
- `j` or down arrow: scroll recent results newer.
- `PageUp` / `PageDown`: scroll by one viewport.
- `g` / `Home`: jump to oldest recent result.
- `G` / `End`: jump to newest recent result.
- `q` / `Ctrl+C`: exit after the benchmark finishes.
- `?`: toggle help.
