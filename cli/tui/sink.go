package tui

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	benchkit "github.com/YoungseokCh/benchkit"

	tea "charm.land/bubbletea/v2"
)

// RecentFilter decides whether a completed case should appear in the recent
// results viewport. It does not affect aggregation or final summaries.
type RecentFilter[T any] func(benchkit.CaseResult[T]) bool

// Sink receives benchmark events and drives the interactive Bubble Tea TUI.
type Sink[T any] struct {
	out          io.Writer
	in           io.Reader
	cancel       context.CancelFunc
	recentFilter RecentFilter[T]
	program      *tea.Program
	done         chan struct{}
	finished     atomic.Bool
	exited       atomic.Bool
	mu           sync.Mutex
	events       []pendingEvent[T]
	aggregate    any
	hasAggregate bool
}

// NewSink creates an interactive TUI event sink.
func NewSink[T any](out io.Writer, in io.Reader, cancel context.CancelFunc, recentFilter RecentFilter[T]) *Sink[T] {
	return &Sink[T]{out: out, in: in, cancel: cancel, recentFilter: recentFilter}
}

func (s *Sink[T]) SuiteStarted(e benchkit.SuiteEvent) {
	model := newModel[T](e, s.recentFilter)
	options := []tea.ProgramOption{tea.WithOutput(s.out), tea.WithFPS(15)}
	if s.in != nil {
		options = append(options, tea.WithInput(s.in))
	}

	s.program = tea.NewProgram(model, options...)
	s.done = make(chan struct{})
	go func() {
		_, _ = s.program.Run()
		if !s.finished.Load() {
			s.exited.Store(true)
		}
		if s.cancel != nil && s.exited.Load() {
			s.cancel()
		}
		close(s.done)
	}()
	go s.flushLoop()
}

func (s *Sink[T]) CaseStarted(e benchkit.WorkerCaseEvent) {
	s.mu.Lock()
	s.events = append(s.events, pendingEvent[T]{started: &e})
	s.mu.Unlock()
}

func (s *Sink[T]) CaseFinished(e benchkit.WorkerCaseResult[T]) {
	s.mu.Lock()
	s.events = append(s.events, pendingEvent[T]{finished: &e})
	s.mu.Unlock()
}

func (s *Sink[T]) AggregateUpdated(snapshot any) {
	s.mu.Lock()
	s.aggregate = snapshot
	s.hasAggregate = true
	s.mu.Unlock()
}

func (s *Sink[T]) SuiteFinished(summary benchkit.Summary[T]) {
	if s.program == nil || s.done == nil {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}
	s.flushPending()
	s.finished.Store(true)
	s.program.Send(suiteFinishedMsg[T]{summary: summary})
	<-s.done
}

// UserExited reports whether the user quit the TUI before the suite finished.
func (s *Sink[T]) UserExited() bool {
	return s.exited.Load()
}

func (s *Sink[T]) flushLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.flushPending()
		case <-s.done:
			return
		}
	}
}

func (s *Sink[T]) flushPending() {
	if s.program == nil {
		return
	}
	msg, ok := s.drainPending()
	if !ok {
		return
	}
	s.program.Send(msg)
}

func (s *Sink[T]) drainPending() (batchMsg[T], bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 && !s.hasAggregate {
		return batchMsg[T]{}, false
	}
	msg := batchMsg[T]{
		events:       append([]pendingEvent[T](nil), s.events...),
		aggregate:    s.aggregate,
		hasAggregate: s.hasAggregate,
	}
	s.events = s.events[:0]
	s.aggregate = nil
	s.hasAggregate = false
	return msg, true
}
