package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/YoungseokCh/benchkit"
	benchkitcli "github.com/YoungseokCh/benchkit/cli"
)

type coverageOutput struct {
	TotalUnits   int `json:"total_units"`
	CoveredUnits int `json:"covered_units"`
}

type coverageAggregator struct {
	totalUnits   int
	coveredUnits int
}

func (a *coverageAggregator) Observe(result benchkit.CaseResult[coverageOutput]) error {
	a.totalUnits += result.Output.TotalUnits
	a.coveredUnits += result.Output.CoveredUnits
	return nil
}

func (a *coverageAggregator) Finalize(summary benchkit.Summary[coverageOutput]) (any, error) {
	return a.Snapshot(), nil
}

func (a *coverageAggregator) Snapshot() any {
	coverage := 0.0
	if a.totalUnits > 0 {
		coverage = float64(a.coveredUnits) / float64(a.totalUnits)
	}
	return benchkit.Stats{
		{
			Title: "coverage",
			Items: []benchkit.StatItem{
				{Label: "total_units", Value: a.totalUnits},
				{Label: "covered_units", Value: a.coveredUnits},
				{Label: "coverage", Value: coverage},
			},
		},
	}
}

func main() {
	suite := benchkit.Benchmark[coverageOutput]{
		Name:       "coverage-demo",
		Cases:      makeCases(120),
		Aggregator: &coverageAggregator{},
		RunCase: func(ctx context.Context, c benchkit.Case) (benchkit.CaseReport[coverageOutput], error) {
			ms, err := strconv.Atoi(c.Meta["ms"])
			if err != nil {
				return benchkit.CaseReport[coverageOutput]{}, err
			}
			totalUnits, err := strconv.Atoi(c.Meta["total_units"])
			if err != nil {
				return benchkit.CaseReport[coverageOutput]{}, err
			}
			coveredUnits, err := strconv.Atoi(c.Meta["covered_units"])
			if err != nil {
				return benchkit.CaseReport[coverageOutput]{}, err
			}

			timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return benchkit.CaseReport[coverageOutput]{}, ctx.Err()
			case <-timer.C:
				output := coverageOutput{TotalUnits: totalUnits, CoveredUnits: coveredUnits}
				return benchkit.CaseReport[coverageOutput]{
					Output: output,
				}, nil
			}
		},
	}

	err := benchkitcli.CLI[coverageOutput]{Benchmark: suite}.Run(context.Background(), os.Args[1:])
	os.Exit(benchkitcli.ExitCode(err))
}

func makeCases(count int) []benchkit.Case {
	cases := make([]benchkit.Case, 0, count)
	for i := 1; i <= count; i++ {
		totalUnits := 800 + ((i * 137) % 7000)
		coveragePercent := 55 + ((i * 29) % 45)
		coveredUnits := totalUnits * coveragePercent / 100
		ms := 2 * (35 + ((i * 41) % 180))
		tags := []string{"coverage"}

		if coveragePercent >= 85 {
			tags = append(tags, "high")
		} else if coveragePercent < 70 {
			tags = append(tags, "low")
		} else {
			tags = append(tags, "medium")
		}

		cases = append(cases, benchkit.Case{
			Name:        "module-" + leftPad(strconv.Itoa(i), 3),
			Description: strings.Join(tags, ", ") + " coverage sample",
			Tags:        tags,
			Meta: map[string]string{
				"ms":            strconv.Itoa(ms),
				"total_units":   strconv.Itoa(totalUnits),
				"covered_units": strconv.Itoa(coveredUnits),
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
