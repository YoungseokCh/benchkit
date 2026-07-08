package benchkit

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Run executes the benchmark suite with bounded parallelism.
func (b Benchmark[T]) Run(ctx context.Context, opts RunOptions[T]) (Summary[T], error) {
	if err := b.validate(); err != nil {
		return Summary[T]{Name: b.Name}, err
	}

	cases := filterCases(b.Cases, opts.Names, opts.Tags, opts.Match)
	parallel := normalizeParallel(opts.Parallel, len(cases))
	sink := opts.Sink
	if sink == nil {
		sink = noopSink[T]{}
	}

	started := time.Now()
	summary := Summary[T]{
		Name:      b.Name,
		Total:     len(cases),
		StartedAt: started,
		Results:   make([]CaseResult[T], 0, len(cases)),
	}

	sink.SuiteStarted(SuiteEvent{Name: b.Name, Total: len(cases), Parallel: parallel})
	if len(cases) == 0 {
		summary.FinishedAt = time.Now()
		summary.Duration = summary.FinishedAt.Sub(summary.StartedAt).Milliseconds()
		sink.SuiteFinished(summary)
		return summary, nil
	}

	type job struct {
		caseData Case
	}

	jobs := make(chan job)
	results := make(chan CaseResult[T])

	var wg sync.WaitGroup
	for worker := 0; worker < parallel; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for item := range jobs {
				results <- b.runOne(ctx, workerID, item.caseData, sink)
			}
		}(worker)
	}

	go func() {
		defer close(jobs)
		for _, caseData := range cases {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{caseData: caseData}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var runErrs []error
	for result := range results {
		if b.Aggregator != nil {
			if err := b.Aggregator.Observe(result); err != nil {
				runErrs = append(runErrs, fmt.Errorf("aggregate %s: %w", result.Case.Name, err))
			} else {
				sink.AggregateUpdated(b.Aggregator.Snapshot())
			}
		}

		summary.Results = append(summary.Results, result)
		summary.count(result.State)
	}

	if err := ctx.Err(); err != nil {
		runErrs = append(runErrs, err)
	}

	summary.FinishedAt = time.Now()
	summary.Duration = summary.FinishedAt.Sub(summary.StartedAt).Milliseconds()

	if b.Aggregator != nil {
		aggregated, err := b.Aggregator.Finalize(summary)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("finalize aggregation: %w", err))
		} else {
			summary.Aggregated = aggregated
		}
	}

	sink.SuiteFinished(summary)
	return summary, errors.Join(runErrs...)
}

func (b Benchmark[T]) runOne(ctx context.Context, workerID int, c Case, sink EventSink[T]) CaseResult[T] {
	sink.CaseStarted(WorkerCaseEvent{WorkerID: workerID, Case: c})
	started := time.Now()

	report, runErr := safeRun(ctx, b.RunCase, c)
	errText := ""

	if runErr != nil {
		report.State = StateError
		report.Message = runErr.Error()
		errText = runErr.Error()
	} else if report.State == "" {
		report.State = StateDone
	}

	finished := time.Now()
	result := CaseResult[T]{
		Case:       c,
		Output:     report.Output,
		State:      report.State,
		Message:    report.Message,
		Error:      errText,
		StartedAt:  started,
		FinishedAt: finished,
		Duration:   finished.Sub(started).Milliseconds(),
	}
	sink.CaseFinished(WorkerCaseResult[T]{WorkerID: workerID, Result: result})
	return result
}

func (b Benchmark[T]) validate() error {
	if b.Name == "" {
		return errors.New("benchmark name is required")
	}
	if b.RunCase == nil {
		return errors.New("benchmark runner is required")
	}
	for i, c := range b.Cases {
		if c.Name == "" {
			return fmt.Errorf("case %d has empty name", i)
		}
	}
	return nil
}

func (s *Summary[T]) count(state State) {
	switch state {
	case "":
		s.Done++
	case StateDone:
		s.Done++
	case StateSkip:
		s.Skipped++
	case StateError:
		s.Errors++
	default:
		s.Errors++
	}
}

func safeRun[T any](ctx context.Context, runner Runner[T], c Case) (report CaseReport[T], err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return runner(ctx, c)
}

func normalizeParallel(requested int, total int) int {
	if total <= 0 {
		return 1
	}
	if requested <= 0 {
		requested = runtime.NumCPU()
	}
	if requested < 1 {
		requested = 1
	}
	if requested > total {
		return total
	}
	return requested
}

func filterCases(cases []Case, names []string, tags []string, match string) []Case {
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}

	tagSet := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tagSet[tag] = struct{}{}
		}
	}

	var selected []Case
	for _, c := range cases {
		if len(nameSet) > 0 {
			if _, ok := nameSet[c.Name]; !ok {
				continue
			}
		}
		if match != "" && !strings.Contains(c.Name, match) {
			continue
		}
		if len(tagSet) > 0 && !hasAllTags(c.Tags, tagSet) {
			continue
		}
		selected = append(selected, c)
	}
	return selected
}

func hasAllTags(tags []string, required map[string]struct{}) bool {
	have := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		have[tag] = struct{}{}
	}
	for tag := range required {
		if _, ok := have[tag]; !ok {
			return false
		}
	}
	return true
}

type noopSink[T any] struct{}

func (noopSink[T]) SuiteStarted(SuiteEvent)          {}
func (noopSink[T]) CaseStarted(WorkerCaseEvent)      {}
func (noopSink[T]) CaseFinished(WorkerCaseResult[T]) {}
func (noopSink[T]) AggregateUpdated(any)             {}
func (noopSink[T]) SuiteFinished(Summary[T])         {}
