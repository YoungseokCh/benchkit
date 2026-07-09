package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const resultSummaryFile = "summary.json"

type fileResultStore[T any] struct{}

func (fileResultStore[T]) Load(dir string) (Summary[T], error) {
	return loadSummary[T](dir)
}

func (fileResultStore[T]) Save(dir string, summary Summary[T]) error {
	return saveSummary(dir, summary)
}

func defaultResultDir(now time.Time) string {
	return filepath.Join("result", now.Format("20060102-150405"))
}

func resultSummaryPath(dir string) string {
	return filepath.Join(dir, resultSummaryFile)
}

func loadSummary[T any](dir string) (Summary[T], error) {
	path := resultSummaryPath(dir)
	file, err := os.Open(path)
	if err != nil {
		return Summary[T]{}, fmt.Errorf("load previous summary %s: %w", path, err)
	}
	defer file.Close()

	var summary Summary[T]
	if err := json.NewDecoder(file).Decode(&summary); err != nil {
		return Summary[T]{}, fmt.Errorf("decode previous summary %s: %w", path, err)
	}
	return summary, nil
}

func saveSummary[T any](dir string, summary Summary[T]) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create result directory %s: %w", dir, err)
	}

	temp, err := os.CreateTemp(dir, ".summary-*.json")
	if err != nil {
		return fmt.Errorf("create temporary summary in %s: %w", dir, err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		_ = temp.Close()
		return fmt.Errorf("encode summary %s: %w", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close summary %s: %w", tempPath, err)
	}

	path := resultSummaryPath(dir)
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("save summary %s: %w", path, err)
	}
	removeTemp = false
	return nil
}
