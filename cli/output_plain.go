package cli

import (
	"fmt"
	"io"
	"sync"
)

type plainSink[T any] struct {
	out       io.Writer
	mu        sync.Mutex
	aggregate string
}

func newPlainSink[T any](out io.Writer) *plainSink[T] {
	return &plainSink[T]{out: out}
}

func (s *plainSink[T]) SuiteStarted(e SuiteEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.out, "Running %s: %d cases, parallel=%d\n", e.Name, e.Total, e.Parallel)
}

func (s *plainSink[T]) CaseStarted(WorkerCaseEvent) {}

func (s *plainSink[T]) CaseFinished(e WorkerCaseResult[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := e.Result
	line := fmt.Sprintf("[%s] %s (%dms)", r.State, r.Case.Name, r.Duration)
	if r.State == "" {
		line = fmt.Sprintf("%s (%dms)", r.Case.Name, r.Duration)
	}
	if r.Message != "" {
		line += ": " + r.Message
	}
	fmt.Fprintln(s.out, line)
}

func (s *plainSink[T]) AggregateUpdated(snapshot any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aggregate = formatAggregate(snapshot)
}

func (s *plainSink[T]) SuiteFinished(summary Summary[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(
		s.out,
		"Summary: total=%d done=%d errors=%d skipped=%d duration=%dms\n",
		summary.Total,
		summary.Done,
		summary.Errors,
		summary.Skipped,
		summary.Duration,
	)
	if summary.Aggregated != nil {
		fmt.Fprintf(s.out, "Aggregate: %s\n", formatAggregate(summary.Aggregated))
	}
}
