package components

import (
	"fmt"

	benchkit "github.com/YoungseokCh/benchkit"
)

// ResultLine renders one recent result line.
func ResultLine[T any](r benchkit.CaseResult[T]) string {
	if r.State == "" {
		line := fmt.Sprintf("%s (%dms)", r.Case.Name, r.Duration)
		if r.Message != "" {
			line += ": " + r.Message
		}
		return line
	}
	state := string(r.State)
	switch r.State {
	case benchkit.StateDone:
		state = passStyle.Render(state)
	case benchkit.StateError:
		state = errorStyle.Render(state)
	default:
		if r.State != benchkit.StateSkip {
			state = failStyle.Render(state)
		}
	}

	line := fmt.Sprintf("[%s] %s (%dms)", state, r.Case.Name, r.Duration)
	if r.Message != "" {
		line += ": " + r.Message
	}
	return line
}
