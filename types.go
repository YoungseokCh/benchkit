package benchkit

import (
	"context"
	"errors"
	"time"
)

// ErrBenchmarkErrored is returned by CLI.Run when the suite completed but at
// least one case errored.
var ErrBenchmarkErrored = errors.New("benchmark errored")

// State is the framework-level execution state for one benchmark case. Domain
// verdicts such as pass/fail belong in the user-defined Output value. Successful
// cases default to StateDone.
type State string

const (
	StateDone  State = "DONE"
	StateError State = "ERROR"
	StateSkip  State = "SKIP"
)

// Case is one benchmark input. Users can keep the benchmark-specific payload in
// Meta, or close over richer data from their Runner.
type Case struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

// CaseReport is the user-produced result payload for one case.
type CaseReport[T any] struct {
	Output  T      `json:"output,omitempty"`
	State   State  `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// Runner executes one benchmark case and returns the user-defined result
// payload. The framework handles concurrency, timing, aggregation, and
// reporting around it.
type Runner[T any] func(context.Context, Case) (CaseReport[T], error)

// Aggregator receives each completed case and returns an arbitrary
// machine-readable summary at the end of the run. Use this for benchmark-domain
// aggregates such as coverage, precision/recall, latency percentiles, cost, or
// any other result model carried by Output.
type Aggregator[T any] interface {
	Observe(CaseResult[T]) error
	Snapshot() any
	Finalize(Summary[T]) (any, error)
}

// Stats is an optional first-class aggregate display model. Aggregators can
// still return any JSON-marshalable value, but returning Stats lets terminal
// output distinguish compact key/value sections from true tables.
type Stats []Stat

// Stat is one aggregate display section.
type Stat struct {
	Title string     `json:"title,omitempty"`
	Items []StatItem `json:"items,omitempty"`
	Table *StatTable `json:"table,omitempty"`
}

// StatItem is one flat aggregate value.
type StatItem struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// StatTable is an aggregate table with named columns.
type StatTable struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// EventSink receives lifecycle events. Implementations are responsible for
// their own synchronization because case events can be emitted from workers.
type EventSink[T any] interface {
	SuiteStarted(SuiteEvent)
	CaseStarted(WorkerCaseEvent)
	CaseFinished(WorkerCaseResult[T])
	AggregateUpdated(any)
	SuiteFinished(Summary[T])
}

// SuiteEvent describes a benchmark run before cases start.
type SuiteEvent struct {
	Name     string `json:"name"`
	Total    int    `json:"total"`
	Parallel int    `json:"parallel"`
}

// WorkerCaseEvent identifies which stable worker slot is handling a case.
type WorkerCaseEvent struct {
	WorkerID int  `json:"worker_id"`
	Case     Case `json:"case"`
}

// WorkerCaseResult identifies which stable worker slot produced a case result.
type WorkerCaseResult[T any] struct {
	WorkerID int           `json:"worker_id"`
	Result   CaseResult[T] `json:"result"`
}

// Benchmark is the user-defined benchmark suite.
type Benchmark[T any] struct {
	Name       string
	Cases      []Case
	RunCase    Runner[T]
	Aggregator Aggregator[T]
}

// RunOptions controls scheduling and result collection.
type RunOptions[T any] struct {
	Parallel int
	Names    []string
	Tags     []string
	Match    string
	Sink     EventSink[T]
}

// CaseResult is the complete result for one case.
type CaseResult[T any] struct {
	Case       Case      `json:"case"`
	Output     T         `json:"output,omitempty"`
	State      State     `json:"state"`
	Message    string    `json:"message,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Duration   int64     `json:"duration_ms"`
}

// Summary is the framework-level run result.
type Summary[T any] struct {
	Name       string          `json:"name"`
	Total      int             `json:"total"`
	Done       int             `json:"done"`
	Errors     int             `json:"errors"`
	Skipped    int             `json:"skipped"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at"`
	Duration   int64           `json:"duration_ms"`
	Results    []CaseResult[T] `json:"results,omitempty"`
	Aggregated any             `json:"aggregated,omitempty"`
}

// OK reports whether there were no errored cases.
func (s Summary[T]) OK() bool {
	return s.Errors == 0
}
