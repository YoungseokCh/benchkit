package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	tuipkg "github.com/YoungseokCh/benchkit/cli/tui"
)

// CLI wraps a Benchmark with an interactive and machine-friendly command-line
// surface. Programs normally expose it from their own main package:
//
//	os.Exit(benchkitcli.ExitCode(benchkitcli.CLI[MyOutput]{Benchmark: suite}.Run(ctx, os.Args[1:])))
type CLI[T any] struct {
	Benchmark    Benchmark[T]
	RecentFilter RecentFilter[T]
	In           io.Reader
	Out          io.Writer
	Err          io.Writer
}

// RecentFilter decides whether a completed case should appear in the TUI
// recent-results viewport. It does not affect aggregation or final summaries.
type RecentFilter[T any] func(CaseResult[T]) bool

// RecentErrors shows errored cases in the TUI recent-results viewport.
func RecentErrors[T any](result CaseResult[T]) bool {
	return result.State == StateError || result.Error != ""
}

// Run parses args, optionally prompts for case selection, runs the benchmark,
// and prints plain text or JSONL.
func (c CLI[T]) Run(ctx context.Context, args []string) error {
	in := c.In
	if in == nil {
		in = os.Stdin
	}
	out := c.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := c.Err
	if errOut == nil {
		errOut = os.Stderr
	}

	var parallel int
	var caseCSV string
	var tagCSV string
	var match string
	var jsonLines bool
	var interactive bool
	var tui bool

	flags := flag.NewFlagSet(c.Benchmark.Name, flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.IntVar(&parallel, "parallel", 0, "maximum number of cases to run concurrently; 0 uses GOMAXPROCS")
	flags.StringVar(&caseCSV, "case", "", "comma-separated exact case names")
	flags.StringVar(&tagCSV, "tag", "", "comma-separated tags; all listed tags are required")
	flags.StringVar(&match, "match", "", "substring filter for case names")
	flags.BoolVar(&jsonLines, "jsonl", false, "stream machine-readable JSON lines")
	flags.BoolVar(&interactive, "interactive", false, "prompt for case selection before running")
	flags.BoolVar(&tui, "tui", true, "use the interactive terminal UI when stdin and stdout are terminals")

	if err := flags.Parse(args); err != nil {
		return err
	}

	names := splitCSV(caseCSV)
	tags := splitCSV(tagCSV)

	if interactive {
		selected, err := promptCases(in, out, c.Benchmark.Cases)
		if err != nil {
			return err
		}
		if len(selected) > 0 {
			names = selected
		}
	}

	bench := c.Benchmark
	bench.Cases = filterCases(c.Benchmark.Cases, names, tags, match)

	var sink EventSink[T]
	runCtx := ctx
	var cancel context.CancelFunc
	var bubble *tuipkg.Sink[T]
	if jsonLines {
		sink = newJSONLinesSink[T](out)
	} else {
		if tui && isTerminal(out) && isTerminal(in) {
			runCtx, cancel = context.WithCancel(ctx)
			defer cancel()
			var recent tuipkg.RecentFilter[T]
			if c.RecentFilter != nil {
				recent = tuipkg.RecentFilter[T](c.RecentFilter)
			}
			bubble = tuipkg.NewSink[T](out, in, cancel, recent)
			sink = bubble
		} else {
			sink = newPlainSink[T](out)
		}
	}

	summary, err := bench.Run(runCtx, RunOptions[T]{
		Parallel: parallel,
		Sink:     sink,
	})
	userExited := bubble != nil && bubble.UserExited()
	if userExited && errors.Is(err, context.Canceled) {
		err = nil
	}
	if !userExited && err == nil && !summary.OK() {
		err = ErrBenchmarkErrored
	}
	return err
}

// ExitCode maps CLI.Run errors to conventional process exit codes.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrBenchmarkErrored) {
		return 1
	}
	return 2
}

func promptCases(in io.Reader, out io.Writer, cases []Case) ([]string, error) {
	fmt.Fprintln(out, "Select cases to run:")
	for i, c := range cases {
		if c.Description != "" {
			fmt.Fprintf(out, "  %d. %s - %s\n", i+1, c.Name, c.Description)
		} else {
			fmt.Fprintf(out, "  %d. %s\n", i+1, c.Name)
		}
	}
	fmt.Fprint(out, "Enter numbers, ranges, names, or 'all' [all]: ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" || strings.EqualFold(line, "all") {
		return nil, nil
	}

	return parseSelection(line, cases)
}

func parseSelection(input string, cases []Case) ([]string, error) {
	nameByIndex := make(map[int]string, len(cases))
	nameSet := make(map[string]struct{}, len(cases))
	for i, c := range cases {
		nameByIndex[i+1] = c.Name
		nameSet[c.Name] = struct{}{}
	}

	var selected []string
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		if strings.Contains(token, "-") {
			parts := strings.SplitN(token, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range %q", token)
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range %q", token)
			}
			if start > end {
				start, end = end, start
			}
			for i := start; i <= end; i++ {
				name, ok := nameByIndex[i]
				if !ok {
					return nil, fmt.Errorf("case index %d out of range", i)
				}
				selected = append(selected, name)
			}
			continue
		}

		if index, err := strconv.Atoi(token); err == nil {
			name, ok := nameByIndex[index]
			if !ok {
				return nil, fmt.Errorf("case index %d out of range", index)
			}
			selected = append(selected, name)
			continue
		}

		if _, ok := nameSet[token]; !ok {
			return nil, fmt.Errorf("unknown case %q", token)
		}
		selected = append(selected, token)
	}

	return selected, nil
}

func splitCSV(input string) []string {
	if input == "" {
		return nil
	}
	var values []string
	for _, value := range strings.Split(input, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}
