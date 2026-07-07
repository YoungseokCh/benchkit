package cli

import "github.com/YoungseokCh/benchkit"

type Benchmark[T any] = benchkit.Benchmark[T]
type Case = benchkit.Case
type CaseResult[T any] = benchkit.CaseResult[T]
type EventSink[T any] = benchkit.EventSink[T]
type RunOptions[T any] = benchkit.RunOptions[T]
type Summary[T any] = benchkit.Summary[T]
type SuiteEvent = benchkit.SuiteEvent
type WorkerCaseEvent = benchkit.WorkerCaseEvent
type WorkerCaseResult[T any] = benchkit.WorkerCaseResult[T]
type Status = benchkit.Status

const (
	StatusPass  = benchkit.StatusPass
	StatusFail  = benchkit.StatusFail
	StatusError = benchkit.StatusError
	StatusSkip  = benchkit.StatusSkip
)

var ErrBenchmarkFailed = benchkit.ErrBenchmarkFailed
