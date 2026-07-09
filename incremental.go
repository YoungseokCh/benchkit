package benchkit

import (
	"context"
	"errors"
	"fmt"
)

// RunIncremental runs the cases selected by opts, merges those results into a
// previous whole-suite summary, and finalizes the aggregate from the merged
// result set.
//
// Aggregation runs after merging, so Aggregate sees the whole updated result set.
func (b Benchmark[T]) RunIncremental(ctx context.Context, opts RunOptions[T], previous Summary[T]) (Summary[T], error) {
	if previous.Name != "" && previous.Name != b.Name {
		return Summary[T]{Name: b.Name}, fmt.Errorf("previous summary is for %q, not %q", previous.Name, b.Name)
	}

	sink := opts.Sink
	if sink == nil {
		sink = noopSink[T]{}
	}

	partialOpts := opts
	partialOpts.Sink = sink

	partialBench := b
	partialBench.Aggregate = nil

	partial, runErr := partialBench.run(ctx, partialOpts, nil, false)
	if runErr != nil && partial.StartedAt.IsZero() {
		return partial, runErr
	}
	merged := b.mergeSummaries(previous, partial)

	var runErrs []error
	if runErr != nil {
		runErrs = append(runErrs, runErr)
	}
	if b.Aggregate != nil {
		aggregated, err := b.Aggregate(merged)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("finalize aggregation: %w", err))
		} else {
			merged.Aggregated = aggregated
			sink.AggregateUpdated(aggregated)
		}
	}

	sink.SuiteFinished(merged)
	return merged, errors.Join(runErrs...)
}

func (b Benchmark[T]) mergeSummaries(previous Summary[T], partial Summary[T]) Summary[T] {
	previousByName := make(map[string]CaseResult[T], len(previous.Results))
	for _, result := range previous.Results {
		previousByName[result.Case.Name] = result
	}

	partialByName := make(map[string]CaseResult[T], len(partial.Results))
	for _, result := range partial.Results {
		partialByName[result.Case.Name] = result
	}

	merged := Summary[T]{
		Name:       b.Name,
		ResultDir:  partial.ResultDir,
		Total:      len(b.Cases),
		StartedAt:  partial.StartedAt,
		FinishedAt: partial.FinishedAt,
		Duration:   partial.Duration,
		Results:    make([]CaseResult[T], 0, len(b.Cases)),
	}

	for _, c := range b.Cases {
		result, ok := partialByName[c.Name]
		if !ok {
			result, ok = previousByName[c.Name]
		}
		if !ok {
			continue
		}
		merged.Results = append(merged.Results, result)
		merged.count(result.State)
	}

	return merged
}
