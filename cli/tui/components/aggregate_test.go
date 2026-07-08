package components

import (
	"strings"
	"testing"

	benchkit "github.com/YoungseokCh/benchkit"
)

func TestFormatAggregateTableRendersStatItemsAsTable(t *testing.T) {
	got := FormatAggregateTable(benchkit.Stats{
		{
			Title: "verdict",
			Items: []benchkit.StatItem{
				{Label: "passed", Value: 94},
				{Label: "failed", Value: 26},
			},
		},
	}, 80)

	wantLines := []string{
		"verdict",
		"┌────────┬───────┐",
		"│ label  │ value │",
		"├────────┼───────┤",
		"│ passed │ 94    │",
		"│ failed │ 26    │",
		"└────────┴───────┘",
	}
	want := strings.Join(wantLines, "\n")

	if got != want {
		t.Fatalf("FormatAggregateTable() =\n%s\nwant\n%s", got, want)
	}
}
