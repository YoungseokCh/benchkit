package tui

import (
	"strings"
	"time"

	benchkit "github.com/YoungseokCh/benchkit"
	"github.com/YoungseokCh/benchkit/cli/tui/components"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type model[T any] struct {
	name         string
	total        int
	parallel     int
	completed    int
	done         int
	errors       int
	skipped      int
	aggregate    string
	totalCaseMS  int64
	width        int
	height       int
	startedAt    time.Time
	finished     bool
	workers      []components.WorkerSlot
	recent       []string
	recentFilter RecentFilter[T]
	followRecent bool
	viewport     viewport.Model
}

func newModel[T any](e benchkit.SuiteEvent, recentFilter RecentFilter[T]) model[T] {
	model := model[T]{
		name:         e.Name,
		total:        e.Total,
		parallel:     e.Parallel,
		startedAt:    time.Now(),
		workers:      make([]components.WorkerSlot, e.Parallel),
		recentFilter: recentFilter,
		followRecent: true,
		viewport:     viewport.New(),
		width:        80,
		height:       24,
	}
	model.viewport.MouseWheelDelta = 1
	model.configureViewport()
	return model
}

func (m model[T]) Init() tea.Cmd {
	return tick()
}

func (m model[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.configureViewport()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k", "pgup", "b", "u", "ctrl+u", "home":
			m.followRecent = false
		case "end":
			m.followRecent = true
		}
	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			m.followRecent = false
		case tea.MouseWheelDown:
			if m.viewport.AtBottom() {
				m.followRecent = true
			}
		}
	case tea.InterruptMsg:
		return m, tea.Quit
	case tickMsg:
		if !m.finished {
			return m, tick()
		}
	case caseStartedMsg:
		m.applyCaseStarted(msg.event)
	case caseFinishedMsg[T]:
		if m.applyCaseFinished(msg.event) {
			m.refreshRecent()
		}
	case batchMsg[T]:
		needsRecentRefresh := false
		for _, event := range msg.events {
			switch {
			case event.started != nil:
				m.applyCaseStarted(*event.started)
			case event.finished != nil:
				if m.applyCaseFinished(*event.finished) {
					needsRecentRefresh = true
				}
			}
		}
		if needsRecentRefresh {
			m.refreshRecent()
		}
		if msg.hasAggregate {
			m.aggregate = components.FormatAggregateTable(msg.aggregate, m.aggregateWidth())
		}
	case suiteFinishedMsg[T]:
		m.completed = msg.summary.Total
		m.done = msg.summary.Done
		m.errors = msg.summary.Errors
		m.skipped = msg.summary.Skipped
		m.aggregate = components.FormatAggregateTable(msg.summary.Aggregated, m.aggregateWidth())
		m.finished = true
		m.refreshRecent()
	case aggregateUpdatedMsg:
		m.aggregate = components.FormatAggregateTable(msg.snapshot, m.aggregateWidth())
	}

	m.viewport, cmd = m.viewport.Update(msg)
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "down", "j", "pgdown", "space", "f", "d", "ctrl+d", "end":
			if m.viewport.AtBottom() {
				m.followRecent = true
			}
		}
	case tea.MouseWheelMsg:
		if msg.Mouse().Button == tea.MouseWheelDown && m.viewport.AtBottom() {
			m.followRecent = true
		}
	}
	return m, cmd
}

func (m *model[T]) applyCaseStarted(event benchkit.WorkerCaseEvent) {
	if event.WorkerID >= 0 && event.WorkerID < len(m.workers) {
		m.workers[event.WorkerID] = components.WorkerSlot{Name: event.Case.Name, StartedAt: time.Now()}
	}
}

func (m *model[T]) applyCaseFinished(event benchkit.WorkerCaseResult[T]) bool {
	result := event.Result
	if event.WorkerID >= 0 && event.WorkerID < len(m.workers) {
		current := m.workers[event.WorkerID]
		if current.Name == result.Case.Name {
			m.workers[event.WorkerID] = components.WorkerSlot{Name: result.Case.Name, StartedAt: result.StartedAt, FinishedAt: result.FinishedAt}
		}
	}
	m.totalCaseMS += result.Duration
	m.completed++
	switch result.State {
	case benchkit.StateDone:
		m.done++
	case benchkit.StateSkip:
		m.skipped++
	case benchkit.StateError:
		m.errors++
	case "":
		m.done++
	default:
		m.errors++
	}
	if m.recentFilter == nil || m.recentFilter(result) {
		m.recent = append(m.recent, components.ResultLine(result))
		return true
	}
	return false
}

func (m model[T]) View() tea.View {
	m.configureViewport()

	aggregate := ""
	if m.aggregate != "" {
		aggregate = components.Block("", m.aggregate, m.width)
	}

	recent := components.ViewportBlock(m.viewport.View(), m.viewport, m.width, m.viewport.Height())

	sections := []string{
		"",
		components.ProgressBar(components.Progress{
			Name:      m.name,
			Total:     m.total,
			Completed: m.completed,
			Done:      m.done,
			Errors:    m.errors,
			Skipped:   m.skipped,
			StartedAt: m.startedAt,
			ETA:       m.suiteETA(),
			Width:     m.width,
		}),
		"",
		m.workerView(),
	}
	if aggregate != "" {
		sections = append(sections, aggregate)
	}
	sections = append(sections,
		"",
		recent,
		components.Footer(m.finished),
	)
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	view := tea.NewView(content)
	view.AltScreen = true
	// Let the terminal own mouse drag selection so users can copy text from the TUI.
	view.MouseMode = tea.MouseModeNone
	return view
}

func (m *model[T]) configureViewport() {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height - m.fixedRows()
	if height < 1 {
		height = 1
	}
	viewportWidth := width - 3
	if viewportWidth < 1 {
		viewportWidth = 1
	}
	m.viewport.SetWidth(viewportWidth)
	m.viewport.SetHeight(height)
}

func (m model[T]) fixedRows() int {
	rows := 7 + m.workerRows()
	if m.aggregate != "" {
		rows += components.LineCount(components.Block("", m.aggregate, m.width))
	}
	return rows
}

func (m model[T]) workerRows() int {
	return components.WorkerGrid{
		Workers:     m.workers,
		Width:       m.width,
		Completed:   m.completed,
		TotalCaseMS: m.totalCaseMS,
	}.Rows()
}

func (m model[T]) workerView() string {
	return components.WorkerGrid{
		Workers:     m.workers,
		Width:       m.width,
		Completed:   m.completed,
		TotalCaseMS: m.totalCaseMS,
	}.View()
}

func (m model[T]) suiteETA() string {
	elapsed := time.Since(m.startedAt)
	if m.completed > 0 && m.completed < m.total {
		perCase := elapsed / time.Duration(m.completed)
		return components.FormatDuration(perCase * time.Duration(m.total-m.completed))
	}
	if m.completed >= m.total {
		return "done"
	}
	return "calculating"
}

func (m model[T]) aggregateWidth() int {
	if m.width <= 2 {
		return 78
	}
	return m.width - 2
}

func (m *model[T]) refreshRecent() {
	m.viewport.SetContent(strings.Join(m.recent, "\n"))
	if m.followRecent {
		m.viewport.GotoBottom()
	}
}

func tick() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}
