package benchkit

// SummaryAggregator is the default lightweight aggregator. It keeps per-state
// counts across all case results.
type SummaryAggregator[T any] struct {
	ByState map[State]int `json:"by_state"`
}

// Observe records one result.
func (a *SummaryAggregator[T]) Observe(result CaseResult[T]) error {
	if a.ByState == nil {
		a.ByState = make(map[State]int)
	}
	state := result.State
	if state == "" {
		state = StateDone
	}
	a.ByState[state]++

	return nil
}

// Finalize returns the aggregate itself.
func (a *SummaryAggregator[T]) Snapshot() any {
	if a.ByState == nil {
		a.ByState = make(map[State]int)
	}
	var stats Stats

	state := Stat{Title: "state"}
	for _, s := range []State{StateDone, StateError, StateSkip} {
		if count, ok := a.ByState[s]; ok {
			state.Items = append(state.Items, StatItem{Label: string(s), Value: count})
		}
	}
	if len(state.Items) > 0 {
		stats = append(stats, state)
	}

	return stats
}

// Finalize returns the aggregate itself.
func (a *SummaryAggregator[T]) Finalize(Summary[T]) (any, error) {
	return a.Snapshot(), nil
}
