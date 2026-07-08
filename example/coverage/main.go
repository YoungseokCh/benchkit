package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/YoungseokCh/benchkit"
	benchkitcli "github.com/YoungseokCh/benchkit/cli"
)

type coverageFile struct {
	Path       string
	TotalUnits int
}

type fileCoverageOutput struct {
	Path         string `json:"path"`
	TotalUnits   int    `json:"total_units"`
	CoveredUnits []int  `json:"covered_units"`
}

type coverageOutput struct {
	Files []fileCoverageOutput `json:"files"`
}

type coverageAggregator struct {
	files map[string]*fileCoverageAggregate
	order []string
}

type fileCoverageAggregate struct {
	totalUnits   int
	covered      []bool
	coveredUnits int
}

func newCoverageAggregator(files []coverageFile) *coverageAggregator {
	aggregator := &coverageAggregator{
		files: make(map[string]*fileCoverageAggregate, len(files)),
	}
	for _, file := range files {
		aggregator.order = append(aggregator.order, file.Path)
		aggregator.files[file.Path] = &fileCoverageAggregate{
			totalUnits: file.TotalUnits,
			covered:    make([]bool, file.TotalUnits),
		}
	}
	return aggregator
}

func (a *coverageAggregator) add(result benchkit.CaseResult[coverageOutput]) error {
	if result.State != benchkit.StateDone {
		return nil
	}
	for _, file := range result.Output.Files {
		aggregate, ok := a.files[file.Path]
		if !ok {
			return fmt.Errorf("unknown coverage file %q", file.Path)
		}
		if file.TotalUnits != aggregate.totalUnits {
			return fmt.Errorf("%s total_units changed from %d to %d", file.Path, aggregate.totalUnits, file.TotalUnits)
		}
		for _, unit := range file.CoveredUnits {
			if unit < 0 || unit >= aggregate.totalUnits {
				return fmt.Errorf("%s covered unit %d is outside [0,%d)", file.Path, unit, aggregate.totalUnits)
			}
			if !aggregate.covered[unit] {
				aggregate.covered[unit] = true
				aggregate.coveredUnits++
			}
		}
	}
	return nil
}

func (a *coverageAggregator) stats() benchkit.Stats {
	totalUnits := 0
	coveredUnits := 0
	rows := make([][]any, 0, len(a.order))
	for _, path := range a.order {
		file := a.files[path]
		totalUnits += file.totalUnits
		coveredUnits += file.coveredUnits
		rows = append(rows, []any{
			path,
			file.totalUnits,
			file.coveredUnits,
			formatPercent(file.coverage()),
		})
	}
	coverage := 0.0
	if totalUnits > 0 {
		coverage = float64(coveredUnits) / float64(totalUnits)
	}
	return benchkit.Stats{
		{
			Title: "coverage",
			Items: []benchkit.StatItem{
				{Label: "files", Value: len(a.order)},
				{Label: "total_units", Value: totalUnits},
				{Label: "covered_units", Value: coveredUnits},
				{Label: "coverage", Value: coverage},
			},
			Table: &benchkit.StatTable{
				Columns: []string{"file", "total", "covered", "coverage"},
				Rows:    rows,
			},
		},
	}
}

func (f fileCoverageAggregate) coverage() float64 {
	if f.totalUnits == 0 {
		return 0
	}
	return float64(f.coveredUnits) / float64(f.totalUnits)
}

func coverageAggregate(files []coverageFile) benchkit.AggregateFunc[coverageOutput] {
	return func(summary benchkit.Summary[coverageOutput]) (any, error) {
		aggregator := newCoverageAggregator(files)
		for _, result := range summary.Results {
			if err := aggregator.add(result); err != nil {
				return nil, err
			}
		}
		return aggregator.stats(), nil
	}
}

func main() {
	files := []coverageFile{
		{Path: "api/server.go", TotalUnits: 96},
		{Path: "cli/runner.go", TotalUnits: 72},
		{Path: "storage/cache.go", TotalUnits: 64},
		{Path: "tui/model.go", TotalUnits: 48},
	}
	cases, outputs := makeCases(120, files)

	suite := benchkit.Benchmark[coverageOutput]{
		Name:      "coverage-demo",
		Cases:     cases,
		Aggregate: coverageAggregate(files),
		RunCase: func(ctx context.Context, c benchkit.Case) (benchkit.CaseReport[coverageOutput], error) {
			ms, err := strconv.Atoi(c.Meta["ms"])
			if err != nil {
				return benchkit.CaseReport[coverageOutput]{}, err
			}
			output, ok := outputs[c.Name]
			if !ok {
				return benchkit.CaseReport[coverageOutput]{}, fmt.Errorf("missing coverage output for %s", c.Name)
			}

			timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return benchkit.CaseReport[coverageOutput]{}, ctx.Err()
			case <-timer.C:
				return benchkit.CaseReport[coverageOutput]{
					Output: output,
				}, nil
			}
		},
	}

	err := benchkitcli.CLI[coverageOutput]{Benchmark: suite}.Run(context.Background(), os.Args[1:])
	os.Exit(benchkitcli.ExitCode(err))
}

func makeCases(count int, files []coverageFile) ([]benchkit.Case, map[string]coverageOutput) {
	cases := make([]benchkit.Case, 0, count)
	outputs := make(map[string]coverageOutput, count)
	for i := 1; i <= count; i++ {
		name := "scenario-" + leftPad(strconv.Itoa(i), 3)
		touchedFiles := 1 + (i % 3)
		output := coverageOutput{Files: make([]fileCoverageOutput, 0, touchedFiles)}
		coveredUnitCount := 0
		for offset := 0; offset < touchedFiles; offset++ {
			file := files[(i+offset)%len(files)]
			unitCount := 2 + ((i*5 + offset*3) % 8)
			coveredUnitCount += unitCount
			output.Files = append(output.Files, fileCoverageOutput{
				Path:         file.Path,
				TotalUnits:   file.TotalUnits,
				CoveredUnits: makeCoveredUnits(file.TotalUnits, i+offset*31, unitCount),
			})
		}
		ms := 2 * (35 + ((i * 41) % 180))
		tags := []string{"coverage"}

		if coveredUnitCount >= 18 {
			tags = append(tags, "high")
		} else if coveredUnitCount < 12 {
			tags = append(tags, "low")
		} else {
			tags = append(tags, "medium")
		}

		cases = append(cases, benchkit.Case{
			Name:        name,
			Description: strings.Join(tags, ", ") + " coverage sample",
			Tags:        tags,
			Meta: map[string]string{
				"ms": strconv.Itoa(ms),
			},
		})
		outputs[name] = output
	}
	return cases, outputs
}

func makeCoveredUnits(totalUnits int, seed int, target int) []int {
	covered := make([]bool, totalUnits)
	units := make([]int, 0, target)
	for offset := 0; len(units) < target; offset++ {
		unit := (seed*37 + offset*17) % totalUnits
		if covered[unit] {
			continue
		}
		covered[unit] = true
		units = append(units, unit)
	}
	return units
}

func leftPad(value string, width int) string {
	for len(value) < width {
		value = "0" + value
	}
	return value
}

func formatPercent(value float64) string {
	return strconv.FormatFloat(value*100, 'f', 1, 64) + "%"
}
