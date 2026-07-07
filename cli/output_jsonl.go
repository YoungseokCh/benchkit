package cli

import (
	"encoding/json"
	"io"
	"sync"
)

type jsonLinesSink[T any] struct {
	out io.Writer
	mu  sync.Mutex
}

func newJSONLinesSink[T any](out io.Writer) *jsonLinesSink[T] {
	return &jsonLinesSink[T]{out: out}
}

func (s *jsonLinesSink[T]) SuiteStarted(e SuiteEvent) {
	s.write(map[string]any{"event": "suite_started", "suite": e})
}

func (s *jsonLinesSink[T]) CaseStarted(e WorkerCaseEvent) {
	s.write(map[string]any{"event": "case_started", "worker_id": e.WorkerID, "case": e.Case})
}

func (s *jsonLinesSink[T]) CaseFinished(e WorkerCaseResult[T]) {
	s.write(map[string]any{"event": "case_finished", "worker_id": e.WorkerID, "result": e.Result})
}

func (s *jsonLinesSink[T]) AggregateUpdated(snapshot any) {
	s.write(map[string]any{"event": "aggregate_updated", "aggregate": snapshot})
}

func (s *jsonLinesSink[T]) SuiteFinished(summary Summary[T]) {
	s.write(map[string]any{"event": "suite_finished", "summary": summary})
}

func (s *jsonLinesSink[T]) write(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = json.NewEncoder(s.out).Encode(v)
}
