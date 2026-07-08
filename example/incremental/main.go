package main

import (
	"context"
	"os"
	"strconv"

	"github.com/YoungseokCh/benchkit"
	benchkitcli "github.com/YoungseokCh/benchkit/cli"
)

type scoreOutput struct {
	Score int `json:"score"`
}

func scoreAggregate(summary benchkit.Summary[scoreOutput]) (any, error) {
	total := 0
	rows := make([][]any, 0, len(summary.Results))
	for _, result := range summary.Results {
		if result.State != benchkit.StateDone {
			continue
		}
		total += result.Output.Score
		rows = append(rows, []any{result.Case.Name, result.Output.Score})
	}

	return benchkit.Stats{
		{
			Title: "scores",
			Items: []benchkit.StatItem{
				{Label: "total", Value: total},
			},
			Table: &benchkit.StatTable{
				Columns: []string{"case", "score"},
				Rows:    rows,
			},
		},
	}, nil
}

func main() {
	suite := benchkit.Benchmark[scoreOutput]{
		Name: "incremental-demo",
		Cases: []benchkit.Case{
			{Name: "alpha", Tags: []string{"demo"}},
			{Name: "beta", Tags: []string{"demo"}},
			{Name: "gamma", Tags: []string{"demo"}},
		},
		Aggregate: scoreAggregate,
		RunCase: func(ctx context.Context, c benchkit.Case) (benchkit.CaseReport[scoreOutput], error) {
			select {
			case <-ctx.Done():
				return benchkit.CaseReport[scoreOutput]{}, ctx.Err()
			default:
			}

			return benchkit.CaseReport[scoreOutput]{
				Output: scoreOutput{Score: scoreFor(c.Name)},
			}, nil
		},
	}

	err := benchkitcli.CLI[scoreOutput]{Benchmark: suite}.Run(context.Background(), os.Args[1:])
	os.Exit(benchkitcli.ExitCode(err))
}

func scoreFor(name string) int {
	defaults := map[string]int{
		"alpha": 10,
		"beta":  20,
		"gamma": 30,
	}
	if override := os.Getenv("SCORE_" + name); override != "" {
		score, err := strconv.Atoi(override)
		if err == nil {
			return score
		}
	}
	return defaults[name]
}
