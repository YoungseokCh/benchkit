package benchkit

import "sort"

// SummaryAggregator is the default lightweight aggregator. It keeps per-status
// counts and metric sums across all case verdicts.
type SummaryAggregator[T any] struct {
	ByStatus map[Status]int       `json:"by_status"`
	Metrics  map[string]MetricSum `json:"metrics,omitempty"`
}

// MetricSum is an aggregate of one numeric metric reported by verdicts.
type MetricSum struct {
	Count int     `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
}

// Observe records one result.
func (a *SummaryAggregator[T]) Observe(result CaseResult[T]) error {
	if a.ByStatus == nil {
		a.ByStatus = make(map[Status]int)
	}
	if result.Status != "" {
		a.ByStatus[result.Status]++
	}

	for name, value := range result.Metrics {
		if a.Metrics == nil {
			a.Metrics = make(map[string]MetricSum)
		}
		current := a.Metrics[name]
		if current.Count == 0 || value < current.Min {
			current.Min = value
		}
		if current.Count == 0 || value > current.Max {
			current.Max = value
		}
		current.Count++
		current.Sum += value
		current.Mean = current.Sum / float64(current.Count)
		a.Metrics[name] = current
	}

	return nil
}

// Finalize returns the aggregate itself.
func (a *SummaryAggregator[T]) Snapshot() any {
	if a.ByStatus == nil {
		a.ByStatus = make(map[Status]int)
	}
	var stats Stats

	status := Stat{Title: "status"}
	for _, s := range []Status{StatusPass, StatusFail, StatusError, StatusSkip} {
		if count, ok := a.ByStatus[s]; ok {
			status.Items = append(status.Items, StatItem{Label: string(s), Value: count})
		}
	}
	if len(status.Items) > 0 {
		stats = append(stats, status)
	}

	if len(a.Metrics) > 0 {
		table := &StatTable{
			Columns: []string{"metric", "count", "min", "mean", "max"},
		}
		names := make([]string, 0, len(a.Metrics))
		for name := range a.Metrics {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value := a.Metrics[name]
			table.Rows = append(table.Rows, []any{name, value.Count, value.Min, value.Mean, value.Max})
		}
		stats = append(stats, Stat{Title: "metrics", Table: table})
	}

	return stats
}

// Finalize returns the aggregate itself.
func (a *SummaryAggregator[T]) Finalize(Summary[T]) (any, error) {
	return a.Snapshot(), nil
}
