package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Progress describes the suite-level progress component.
type Progress struct {
	Name      string
	Total     int
	Completed int
	Done      int
	Errors    int
	Skipped   int
	StartedAt time.Time
	ETA       string
	Width     int
}

// ProgressBar renders the full-suite progress line.
func ProgressBar(progress Progress) string {
	prefix := titleStyle.Render(progress.Name) + "  "
	suffix := fmt.Sprintf(
		"  %3d%%  %d/%d  %s  elapsed %s  eta %s",
		percentComplete(progress.Completed, progress.Total),
		progress.Completed,
		progress.Total,
		stateSummary(progress.Done, progress.Errors, progress.Skipped),
		FormatDuration(time.Since(progress.StartedAt)),
		progress.ETA,
	)
	width := progress.Width - lipgloss.Width(prefix) - lipgloss.Width(suffix)
	if width < 10 {
		width = 10
	}

	filled := 0
	if progress.Total > 0 {
		filled = progress.Completed * width / progress.Total
	}
	if filled > width {
		filled = width
	}
	empty := width - filled
	bar := meterDoneStyle.Render(strings.Repeat("█", filled)) + meterTodoStyle.Render(strings.Repeat("░", empty))
	return prefix + bar + suffix
}

func stateSummary(done int, errors int, skipped int) string {
	return fmt.Sprintf("done %d  err %d  skip %d", done, errors, skipped)
}

func percentComplete(completed int, total int) int {
	if total <= 0 {
		return 0
	}
	return completed * 100 / total
}
