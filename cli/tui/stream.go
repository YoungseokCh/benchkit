package tui

import (
	"encoding/json"
	"fmt"

	benchkit "github.com/YoungseokCh/benchkit"
)

type streamLine struct {
	Plain string
	JSON  string
	Hide  bool
}

func jsonLine(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func caseFinishedStreamLine[T any](e benchkit.WorkerCaseResult[T]) streamLine {
	result := e.Result
	payload := plainOutput(result.Output)
	nameAndOutput := result.Case.Name
	if payload != "" {
		nameAndOutput += " " + payload
	}
	plain := fmt.Sprintf("[%s] %s (%dms)", result.State, nameAndOutput, result.Duration)
	if result.State == "" || result.State == benchkit.StateDone {
		plain = fmt.Sprintf("%s (%dms)", nameAndOutput, result.Duration)
	}
	if result.Message != "" {
		plain += ": " + result.Message
	}
	return streamLine{
		Plain: plain,
		JSON:  jsonLine(map[string]any{"event": "case_finished", "worker_id": e.WorkerID, "result": e.Result}),
	}
}

func plainOutput(output any) string {
	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Sprintf("output=%v", output)
	}
	text := string(data)
	switch text {
	case "null", "{}":
		return ""
	default:
		return text
	}
}
