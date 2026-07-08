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
	stream       []streamLine
	streamFilter StreamFilter[T]
	followRecent bool
	showHidden   bool
	activeTab    viewTab
	streamMode   streamMode
	viewport     viewport.Model
}

type viewTab int

const (
	viewTabStats viewTab = iota
	viewTabStream
)

type streamMode int

const (
	streamModePlain streamMode = iota
	streamModeJSON
)

func newModel[T any](e benchkit.SuiteEvent, streamFilter StreamFilter[T]) model[T] {
	model := model[T]{
		name:         e.Name,
		total:        e.Total,
		parallel:     e.Parallel,
		startedAt:    time.Now(),
		workers:      make([]components.WorkerSlot, e.Parallel),
		streamFilter: streamFilter,
		followRecent: true,
		activeTab:    viewTabStream,
		streamMode:   streamModePlain,
		viewport:     viewport.New(),
		width:        80,
		height:       24,
	}
	model.viewport.MouseWheelDelta = 1
	model.configureViewport()
	model.refreshPanel()
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
		case "tab":
			m.nextTab()
			m.configureViewport()
			m.refreshPanel()
			m.resetViewportForTab()
		case "shift+tab":
			m.previousTab()
			m.configureViewport()
			m.refreshPanel()
			m.resetViewportForTab()
		case "1", "s":
			m.activeTab = viewTabStats
			m.configureViewport()
			m.refreshPanel()
			m.resetViewportForTab()
		case "2", "o":
			m.activeTab = viewTabStream
			m.configureViewport()
			m.refreshPanel()
			m.resetViewportForTab()
		case "m":
			m.toggleStreamMode()
			m.refreshPanel()
		case "p":
			m.streamMode = streamModePlain
			m.refreshPanel()
		case "r":
			m.streamMode = streamModeJSON
			m.refreshPanel()
		case "a":
			m.showHidden = !m.showHidden
			m.refreshPanel()
		case "up", "k", "pgup", "b", "u", "ctrl+u", "home":
			if m.activeTab == viewTabStream {
				m.followRecent = false
			}
		case "end":
			if m.activeTab == viewTabStream {
				m.followRecent = true
			}
		}
	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			if m.activeTab == viewTabStream {
				m.followRecent = false
			}
		case tea.MouseWheelDown:
			if m.activeTab == viewTabStream && m.viewport.AtBottom() {
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
		m.applyCaseFinished(msg.event)
		m.refreshPanel()
	case batchMsg[T]:
		needsStreamRefresh := false
		for _, event := range msg.events {
			switch {
			case event.started != nil:
				m.applyCaseStarted(*event.started)
			case event.finished != nil:
				m.applyCaseFinished(*event.finished)
				needsStreamRefresh = true
			}
		}
		if needsStreamRefresh {
			m.refreshPanel()
		}
		if msg.hasAggregate {
			m.aggregate = components.FormatAggregateTable(msg.aggregate, m.aggregateWidth())
			m.refreshPanel()
		}
	case suiteFinishedMsg[T]:
		m.completed = msg.summary.Total
		m.done = msg.summary.Done
		m.errors = msg.summary.Errors
		m.skipped = msg.summary.Skipped
		m.aggregate = components.FormatAggregateTable(msg.summary.Aggregated, m.aggregateWidth())
		m.finished = true
		m.refreshPanel()
	case aggregateUpdatedMsg:
		m.aggregate = components.FormatAggregateTable(msg.snapshot, m.aggregateWidth())
		m.refreshPanel()
	}

	handledViewportInput := false
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		handledViewportInput = m.handleViewportKey(msg)
	case tea.MouseWheelMsg:
		handledViewportInput = m.handleViewportMouse(msg)
	}
	if !handledViewportInput {
		m.viewport, cmd = m.viewport.Update(msg)
	}
	return m, cmd
}

func (m *model[T]) applyCaseStarted(event benchkit.WorkerCaseEvent) {
	if event.WorkerID >= 0 && event.WorkerID < len(m.workers) {
		m.workers[event.WorkerID] = components.WorkerSlot{Name: event.Case.Name, StartedAt: time.Now()}
	}
}

func (m *model[T]) applyCaseFinished(event benchkit.WorkerCaseResult[T]) {
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
	line := caseFinishedStreamLine(event)
	line.Hide = !m.showCompletedResult(result)
	m.stream = append(m.stream, line)
}

func (m model[T]) View() tea.View {
	m.configureViewport()

	panel := m.panelView()

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
		m.workerGrid().View(),
		"",
		panel,
	}
	sections = append(sections,
		components.Footer(m.finished, "tab switches view  1/s stats  2/o stream  a reveal hidden  m mode  p plain  r json"),
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
	viewportWidth, height := components.PanelViewportSize(width, m.panelHeight())
	m.viewport.SetWidth(viewportWidth)
	m.viewport.SetHeight(height)
}

func (m model[T]) workerGrid() components.WorkerGrid {
	return components.WorkerGrid{
		Workers:     m.workers,
		Width:       m.width,
		Completed:   m.completed,
		TotalCaseMS: m.totalCaseMS,
	}
}

func (m model[T]) panelHeight() int {
	height := m.height - 6 - m.workerGrid().Rows()
	if height < 3 {
		height = 3
	}
	return height
}

func (m model[T]) panelView() string {
	switch m.activeTab {
	case viewTabStream:
		return components.Panel{
			Title:    "stream",
			Tabs:     m.tabs(),
			Width:    m.width,
			Height:   m.panelHeight(),
			Viewport: &m.viewport,
		}.View()
	case viewTabStats:
		return components.Panel{
			Title:    "stats",
			Tabs:     m.tabs(),
			Width:    m.width,
			Height:   m.panelHeight(),
			Viewport: &m.viewport,
		}.View()
	default:
		return ""
	}
}

func (m model[T]) tabs() []components.Tab {
	return []components.Tab{
		{Label: "stats", Active: m.activeTab == viewTabStats},
		{Label: m.streamTabLabel(), Active: m.activeTab == viewTabStream},
	}
}

func (m model[T]) statsBody() string {
	stats := m.aggregate
	if stats == "" {
		stats = "waiting for stats"
	}
	return stats
}

func (m model[T]) streamTabLabel() string {
	suffix := ""
	if m.hiddenStreamCount() > 0 && !m.showHidden {
		suffix = ":filtered"
	}
	switch m.streamMode {
	case streamModeJSON:
		return "stream:json" + suffix
	default:
		return "stream:plain" + suffix
	}
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

func (m *model[T]) refreshPanel() {
	switch m.activeTab {
	case viewTabStats:
		m.viewport.SetContent(m.statsBody())
	default:
		m.viewport.SetContent(strings.Join(m.streamLines(), "\n"))
		if m.followRecent {
			m.viewport.GotoBottom()
		}
	}
}

func (m *model[T]) resetViewportForTab() {
	switch m.activeTab {
	case viewTabStats:
		m.viewport.GotoTop()
	default:
		m.followRecent = true
		m.viewport.GotoBottom()
	}
}

func (m *model[T]) handleViewportKey(msg tea.KeyPressMsg) bool {
	m.configureViewport()
	switch msg.String() {
	case "down", "j":
		m.viewport.ScrollDown(1)
	case "up", "k":
		m.viewport.ScrollUp(1)
	case "pgdown", "pagedown", "space", "f":
		m.viewport.PageDown()
	case "pgup", "pageup", "b":
		m.viewport.PageUp()
	case "d", "ctrl+d":
		m.viewport.HalfPageDown()
	case "u", "ctrl+u":
		m.viewport.HalfPageUp()
	case "home", "g":
		m.viewport.GotoTop()
	case "end", "G":
		m.viewport.GotoBottom()
	default:
		switch msg.Key().Code {
		case tea.KeyDown:
			m.viewport.ScrollDown(1)
		case tea.KeyUp:
			m.viewport.ScrollUp(1)
		case tea.KeyPgDown:
			m.viewport.PageDown()
		case tea.KeyPgUp:
			m.viewport.PageUp()
		case tea.KeyHome:
			m.viewport.GotoTop()
		case tea.KeyEnd:
			m.viewport.GotoBottom()
		default:
			return false
		}
	}
	m.updateFollowRecentAfterScroll()
	return true
}

func (m *model[T]) handleViewportMouse(msg tea.MouseWheelMsg) bool {
	m.configureViewport()
	switch msg.Mouse().Button {
	case tea.MouseWheelUp:
		m.viewport.ScrollUp(m.viewport.MouseWheelDelta)
	case tea.MouseWheelDown:
		m.viewport.ScrollDown(m.viewport.MouseWheelDelta)
	default:
		return false
	}
	m.updateFollowRecentAfterScroll()
	return true
}

func (m *model[T]) updateFollowRecentAfterScroll() {
	if m.activeTab != viewTabStream {
		return
	}
	m.followRecent = m.viewport.AtBottom()
}

func (m model[T]) streamLines() []string {
	lines := make([]string, 0, len(m.stream))
	for _, line := range m.stream {
		if line.Hide && !m.showHidden {
			continue
		}
		switch m.streamMode {
		case streamModeJSON:
			lines = append(lines, line.JSON)
		default:
			lines = append(lines, line.Plain)
		}
	}
	return lines
}

func (m *model[T]) nextTab() {
	switch m.activeTab {
	case viewTabStats:
		m.activeTab = viewTabStream
	default:
		m.activeTab = viewTabStats
	}
}

func (m *model[T]) previousTab() {
	m.nextTab()
}

func (m *model[T]) toggleStreamMode() {
	switch m.streamMode {
	case streamModePlain:
		m.streamMode = streamModeJSON
	default:
		m.streamMode = streamModePlain
	}
}

func (m model[T]) showCompletedResult(result benchkit.CaseResult[T]) bool {
	if result.State == benchkit.StateError || result.Error != "" {
		return true
	}
	if m.streamFilter == nil {
		return true
	}
	return m.streamFilter(result)
}

func (m model[T]) hiddenStreamCount() int {
	count := 0
	for _, line := range m.stream {
		if line.Hide {
			count++
		}
	}
	return count
}

func tick() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}
