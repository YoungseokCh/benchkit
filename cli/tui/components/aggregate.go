package components

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// FormatAggregateTable renders a structured aggregate snapshot for the TUI.
func FormatAggregateTable(value any, width int) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return string(data)
	}
	if stats, ok := raw.([]any); ok {
		if rendered := formatStatsTable(stats, width); rendered != "" {
			return rendered
		}
	}
	decoded, ok := raw.(map[string]any)
	if !ok {
		return string(data)
	}

	var rows [][2]string
	if values, ok := decoded["by_state"].(map[string]any); ok {
		for _, key := range sortedKeys(values) {
			if value, ok := values[key].(float64); ok {
				rows = append(rows, [2]string{key, formatFloatCompact(value)})
			}
		}
	}

	for _, key := range sortedKeys(decoded) {
		values, ok := decoded[key].(map[string]any)
		if !ok || key == "by_state" {
			continue
		}
		for _, nestedKey := range sortedKeys(values) {
			nested, ok := values[nestedKey].(map[string]any)
			if !ok {
				continue
			}
			if mean, ok := nested["mean"].(float64); ok {
				rows = append(rows, [2]string{nestedKey + " mean", formatFloatCompact(mean)})
			}
		}
	}

	for _, key := range sortedKeys(decoded) {
		switch v := decoded[key].(type) {
		case string:
			rows = append(rows, [2]string{key, v})
		case float64:
			rows = append(rows, [2]string{key, formatFloatCompact(v)})
		case bool:
			rows = append(rows, [2]string{key, strconv.FormatBool(v)})
		}
	}

	if len(rows) == 0 {
		return string(data)
	}
	return formatAggregateRows(rows, width)
}

func formatStatsTable(stats []any, width int) string {
	var sections []string
	for _, raw := range stats {
		stat, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		title, _ := stat["title"].(string)

		var parts []string
		if items, ok := stat["items"].([]any); ok {
			var rows [][]string
			for _, rawItem := range items {
				item, ok := rawItem.(map[string]any)
				if !ok {
					continue
				}
				label, _ := item["label"].(string)
				if label == "" {
					continue
				}
				rows = append(rows, []string{label, formatStatValue(item["value"])})
			}
			if len(rows) > 0 {
				parts = append(parts, formatTable([]string{"label", "value"}, rows, width))
			}
		}

		if table, ok := stat["table"].(map[string]any); ok {
			rendered := formatStatTable(table, width)
			if rendered != "" {
				parts = append(parts, rendered)
			}
		}

		if len(parts) == 0 {
			continue
		}
		section := strings.Join(parts, "\n")
		if title != "" {
			section = title + "\n" + section
		}
		sections = append(sections, section)
	}
	return strings.Join(sections, "\n")
}

func formatStatTable(table map[string]any, width int) string {
	rawColumns, ok := table["columns"].([]any)
	if !ok || len(rawColumns) == 0 {
		return ""
	}
	columns := make([]string, 0, len(rawColumns))
	for _, raw := range rawColumns {
		columns = append(columns, fmt.Sprint(raw))
	}
	rawRows, ok := table["rows"].([]any)
	if !ok {
		return ""
	}
	rows := make([][]string, 0, len(rawRows))
	for _, rawRow := range rawRows {
		values, ok := rawRow.([]any)
		if !ok {
			continue
		}
		row := make([]string, len(columns))
		for i := range columns {
			if i < len(values) {
				row[i] = formatStatValue(values[i])
			}
		}
		rows = append(rows, row)
	}
	return formatTable(columns, rows, width)
}

func formatTable(columns []string, rows [][]string, width int) string {
	widths := make([]int, len(columns))
	for i, column := range columns {
		widths[i] = lipgloss.Width(column)
	}
	for _, row := range rows {
		for i, value := range row {
			if i < len(widths) && lipgloss.Width(value) > widths[i] {
				widths[i] = lipgloss.Width(value)
			}
		}
	}

	renderRow := func(row []string) string {
		cells := make([]string, len(widths))
		for i := range widths {
			value := ""
			if i < len(row) {
				value = row[i]
			}
			cells[i] = " " + padRight(value, widths[i]) + " "
		}
		return "│" + strings.Join(cells, "│") + "│"
	}

	border := func(left string, join string, right string) string {
		parts := make([]string, len(widths))
		for i, width := range widths {
			parts[i] = strings.Repeat("─", width+2)
		}
		return left + strings.Join(parts, join) + right
	}

	lines := []string{
		border("┌", "┬", "┐"),
		renderRow(columns),
		border("├", "┼", "┤"),
	}
	for _, row := range rows {
		lines = append(lines, renderRow(row))
	}
	lines = append(lines, border("└", "┴", "┘"))
	return strings.Join(lines, "\n")
}

func formatStatValue(value any) string {
	switch v := value.(type) {
	case float64:
		return formatFloatCompact(v)
	case float32:
		return formatFloatCompact(float64(v))
	default:
		return fmt.Sprint(v)
	}
}

func formatAggregateRows(rows [][2]string, width int) string {
	if width <= 0 {
		width = 80
	}
	labelWidth := 0
	valueWidth := 0
	for _, row := range rows {
		if w := lipgloss.Width(row[0]); w > labelWidth {
			labelWidth = w
		}
		if w := lipgloss.Width(row[1]); w > valueWidth {
			valueWidth = w
		}
	}
	cellWidth := labelWidth + 1 + valueWidth
	if cellWidth < 10 {
		cellWidth = 10
	}
	gap := 3
	cols := (width + gap) / (cellWidth + gap)
	if cols < 1 {
		cols = 1
	}

	var lines []string
	for i := 0; i < len(rows); i += cols {
		end := i + cols
		if end > len(rows) {
			end = len(rows)
		}
		var cells []string
		for _, row := range rows[i:end] {
			cell := padRight(row[0], labelWidth) + " " + padLeft(row[1], valueWidth)
			cells = append(cells, padRight(cell, cellWidth))
		}
		lines = append(lines, strings.Join(cells, strings.Repeat(" ", gap)))
	}
	return strings.Join(lines, "\n")
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatFloatCompact(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', 3, 64)
}
