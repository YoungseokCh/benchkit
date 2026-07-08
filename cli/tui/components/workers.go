package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// WorkerSlot is one stable runner slot in the TUI.
type WorkerSlot struct {
	Name       string
	StartedAt  time.Time
	FinishedAt time.Time
}

// WorkerGrid describes the runner status grid.
type WorkerGrid struct {
	Workers     []WorkerSlot
	Width       int
	Completed   int
	TotalCaseMS int64
}

const (
	workerBaseBarWidth = 18
	workerCardHeight   = 5
	workerGridGap      = 2
	workerMinCellWidth = workerBaseBarWidth + 12
)

// Rows returns how many terminal rows the worker grid occupies.
func (g WorkerGrid) Rows() int {
	limit := g.workerLimit()
	cols := g.workerColumns()
	cardRows := (limit + cols - 1) / cols
	return cardRows * workerCardHeight
}

// View renders the worker status grid.
func (g WorkerGrid) View() string {
	var lines []string
	limit := g.workerLimit()
	cols := g.workerColumns()
	cardRows := (limit + cols - 1) / cols
	cellWidth := g.workerCellWidth()
	for row := 0; row < cardRows; row++ {
		var cells []string
		count := cols
		if remaining := limit - row*cols; remaining < count {
			count = remaining
		}
		for col := 0; col < count; col++ {
			index := row*cols + col
			if index >= limit || index >= len(g.Workers) {
				continue
			}
			cells = append(cells, lipgloss.NewStyle().Width(cellWidth).Render(g.workerCell(index, cellWidth)))
		}
		lines = append(lines, strings.Split(joinWorkerCards(cells), "\n")...)
	}
	return strings.Join(lines, "\n")
}

func (g WorkerGrid) workerLimit() int {
	limit := len(g.Workers)
	if limit < 1 {
		limit = 1
	}
	return limit
}

func (g WorkerGrid) workerColumns() int {
	width := g.Width
	if width <= 0 {
		width = 80
	}
	cols := (width + workerGridGap) / (workerMinCellWidth + workerGridGap)
	if cols < 1 {
		cols = 1
	}
	limit := g.workerLimit()
	if cols > limit {
		cols = limit
	}
	return cols
}

func (g WorkerGrid) workerCellWidth() int {
	cols := g.workerColumns()
	if cols < 1 {
		return workerMinCellWidth
	}
	width := g.Width
	if width <= 0 {
		width = 80
	}
	available := width - (cols-1)*workerGridGap
	if available < cols*workerMinCellWidth {
		return workerMinCellWidth
	}
	return available / cols
}

func (g WorkerGrid) workerCell(index int, width int) string {
	slot := g.Workers[index]
	innerWidth := width - 2
	barWidth := innerWidth
	if barWidth < 8 {
		barWidth = 8
	}

	if slot.Name == "" || !slot.FinishedAt.IsZero() {
		title := fmt.Sprintf("runner %02d", index)
		bar := workerProgressBar(0, barWidth)
		return workerCard(title, "idle", bar, "waiting", width)
	}

	elapsed := time.Since(slot.StartedAt)
	if !slot.FinishedAt.IsZero() {
		elapsed = slot.FinishedAt.Sub(slot.StartedAt)
	}

	progress := 0.0
	estimate := g.estimatedCaseDuration()
	if estimate > 0 {
		progress = float64(elapsed) / float64(estimate)
		if progress > 1 {
			progress = 1
		}
	}

	eta := time.Duration(0)
	if estimate > 0 {
		eta = estimate - elapsed
		if eta < 0 {
			eta = 0
		}
	}

	title := fmt.Sprintf("runner %02d", index)
	bar := workerProgressBar(progress, barWidth)
	detail := "elapsed " + FormatDuration(elapsed)
	if estimate > 0 {
		detail += "  eta " + FormatDuration(eta)
	} else {
		detail += "  eta ..."
	}
	return workerCard(title, slot.Name, bar, detail, width)
}

func (g WorkerGrid) estimatedCaseDuration() time.Duration {
	if g.Completed == 0 || g.TotalCaseMS <= 0 {
		return 0
	}
	return time.Duration(g.TotalCaseMS/int64(g.Completed)) * time.Millisecond
}

func joinWorkerCards(cards []string) string {
	if len(cards) == 0 {
		return ""
	}
	if len(cards) == 1 {
		return cards[0]
	}
	spacer := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", workerGridGap)+"\n", workerCardHeight), "\n")
	parts := make([]string, 0, len(cards)*2-1)
	for i, card := range cards {
		if i > 0 {
			parts = append(parts, spacer)
		}
		parts = append(parts, card)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func workerProgressBar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}
	return meterDoneStyle.Render(strings.Repeat("█", filled)) + meterTodoStyle.Render(strings.Repeat("░", width-filled))
}

func workerCard(title string, caseLine string, progressLine string, timingLine string, width int) string {
	if width < 22 {
		width = 22
	}
	innerWidth := width - 2
	title = truncate(title, innerWidth)
	topFill := innerWidth - lipgloss.Width(title)
	if topFill < 0 {
		topFill = 0
	}

	top := mutedStyle.Render("╭" + title + strings.Repeat("─", topFill) + "╮")
	caseRow := mutedStyle.Render("│") + padRight(truncate(caseLine, innerWidth), innerWidth) + mutedStyle.Render("│")
	middle := mutedStyle.Render("│") + padRight(progressLine, innerWidth) + mutedStyle.Render("│")
	bottom := mutedStyle.Render("│") + padRight(truncate(timingLine, innerWidth), innerWidth) + mutedStyle.Render("│")
	foot := mutedStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")
	return strings.Join([]string{top, caseRow, middle, bottom, foot}, "\n")
}
