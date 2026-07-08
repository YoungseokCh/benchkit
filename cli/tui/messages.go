package tui

import benchkit "github.com/YoungseokCh/benchkit"

type caseStartedMsg struct {
	event benchkit.WorkerCaseEvent
}

type caseFinishedMsg[T any] struct {
	event benchkit.WorkerCaseResult[T]
}

type suiteFinishedMsg[T any] struct {
	summary benchkit.Summary[T]
}

type aggregateUpdatedMsg struct {
	snapshot any
}

type batchMsg[T any] struct {
	events       []pendingEvent[T]
	aggregate    any
	hasAggregate bool
}

type pendingEvent[T any] struct {
	started  *benchkit.WorkerCaseEvent
	finished *benchkit.WorkerCaseResult[T]
}

type tickMsg struct{}
