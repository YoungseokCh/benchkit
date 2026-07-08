package main

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/YoungseokCh/benchkit"
	benchkitcli "github.com/YoungseokCh/benchkit/cli"
)

type jobOutput struct {
	WorkUnits int  `json:"work_units"`
	LatencyMS int  `json:"latency_ms"`
	Passed    bool `json:"passed"`
}

type jobAggregator struct {
	passed int
	failed int
}

func (a *jobAggregator) Observe(result benchkit.CaseResult[jobOutput]) error {
	if result.State != benchkit.StateDone {
		return nil
	}
	if result.Output.Passed {
		a.passed++
	} else {
		a.failed++
	}
	return nil
}

func (a *jobAggregator) Snapshot() any {
	return benchkit.Stats{
		{
			Title: "verdict",
			Items: []benchkit.StatItem{
				{Label: "passed", Value: a.passed},
				{Label: "failed", Value: a.failed},
			},
		},
	}
}

func (a *jobAggregator) Finalize(benchkit.Summary[jobOutput]) (any, error) {
	return a.Snapshot(), nil
}

func main() {
	suite := benchkit.Benchmark[jobOutput]{
		Name:       "testing-demo",
		Cases:      makeCases(120),
		Aggregator: &jobAggregator{},
		RunCase: func(ctx context.Context, c benchkit.Case) (benchkit.CaseReport[jobOutput], error) {
			ms, err := strconv.Atoi(c.Meta["ms"])
			if err != nil {
				return benchkit.CaseReport[jobOutput]{}, err
			}
			if ms < 0 {
				return benchkit.CaseReport[jobOutput]{}, errors.New("negative duration")
			}
			limit, err := strconv.Atoi(c.Meta["limit_ms"])
			if err != nil {
				return benchkit.CaseReport[jobOutput]{}, err
			}

			timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return benchkit.CaseReport[jobOutput]{}, ctx.Err()
			case <-timer.C:
				workUnits, err := strconv.Atoi(c.Meta["work_units"])
				if err != nil {
					return benchkit.CaseReport[jobOutput]{}, err
				}
				output := jobOutput{
					WorkUnits: workUnits,
					LatencyMS: ms,
					Passed:    ms <= limit,
				}
				report := benchkit.CaseReport[jobOutput]{
					Output: output,
				}
				return report, nil
			}
		},
	}

	err := benchkitcli.CLI[jobOutput]{
		Benchmark: suite,
		StreamFilter: func(result benchkit.CaseResult[jobOutput]) bool {
			return result.State == benchkit.StateError || !result.Output.Passed
		},
	}.Run(context.Background(), os.Args[1:])
	os.Exit(benchkitcli.ExitCode(err))
}

func makeCases(count int) []benchkit.Case {
	cases := make([]benchkit.Case, 0, count)
	for i := 1; i <= count; i++ {
		latency := 6 * (40 + ((i * 37) % 220))
		limit := 1260
		workUnits := 500 + ((i * 91) % 5000)
		tags := []string{"batch"}

		switch {
		case i <= 10:
			tags = append(tags, "smoke")
		case latency > limit:
			tags = append(tags, "slow")
		default:
			tags = append(tags, "steady")
		}

		if i%15 == 0 {
			tags = append(tags, "large")
			workUnits *= 2
		}

		cases = append(cases, benchkit.Case{
			Name:        "job-" + leftPad(strconv.Itoa(i), 3),
			Description: strings.Join(tags, ", ") + " synthetic workload",
			Tags:        tags,
			Meta: map[string]string{
				"ms":         strconv.Itoa(latency),
				"limit_ms":   strconv.Itoa(limit),
				"work_units": strconv.Itoa(workUnits),
			},
		})
	}
	return cases
}

func leftPad(value string, width int) string {
	for len(value) < width {
		value = "0" + value
	}
	return value
}
