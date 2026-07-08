package benchkit

// StateAggregate returns a lightweight aggregate with per-state counts.
func StateAggregate[T any](summary Summary[T]) (any, error) {
	byState := make(map[State]int)
	for _, result := range summary.Results {
		state := result.State
		if state == "" {
			state = StateDone
		}
		byState[state]++
	}

	var stats Stats
	stateStats := Stat{Title: "state"}
	for _, state := range []State{StateDone, StateError, StateSkip} {
		if count, ok := byState[state]; ok {
			stateStats.Items = append(stateStats.Items, StatItem{Label: string(state), Value: count})
		}
	}
	if len(stateStats.Items) > 0 {
		stats = append(stats, stateStats)
	}

	return stats, nil
}
