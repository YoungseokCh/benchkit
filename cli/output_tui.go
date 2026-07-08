package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type bubbleSink[T any] struct {
	out          io.Writer
	in           io.Reader
	cancel       context.CancelFunc
	recentFilter RecentFilter[T]
	program      *tea.Program
	done         chan struct{}
	finished     atomic.Bool
	exited       atomic.Bool
	mu           sync.Mutex
	events       []bubblePendingEvent[T]
	aggregate    any
	hasAggregate bool
}

func newBubbleSink[T any](out io.Writer, in io.Reader, cancel context.CancelFunc, recentFilter RecentFilter[T]) *bubbleSink[T] {
	return &bubbleSink[T]{out: out, in: in, cancel: cancel, recentFilter: recentFilter}
}

func (s *bubbleSink[T]) SuiteStarted(e SuiteEvent) {
	model := newBubbleModel[T](e, s.recentFilter)
	options := []tea.ProgramOption{tea.WithOutput(s.out), tea.WithFPS(15)}
	if s.in != nil {
		options = append(options, tea.WithInput(s.in))
	}

	s.program = tea.NewProgram(model, options...)
	s.done = make(chan struct{})
	go func() {
		_, _ = s.program.Run()
		if !s.finished.Load() {
			s.exited.Store(true)
		}
		if s.cancel != nil && s.exited.Load() {
			s.cancel()
		}
		close(s.done)
	}()
	go s.flushLoop()
}

func (s *bubbleSink[T]) CaseStarted(e WorkerCaseEvent) {
	s.mu.Lock()
	s.events = append(s.events, bubblePendingEvent[T]{started: &e})
	s.mu.Unlock()
}

func (s *bubbleSink[T]) CaseFinished(e WorkerCaseResult[T]) {
	s.mu.Lock()
	s.events = append(s.events, bubblePendingEvent[T]{finished: &e})
	s.mu.Unlock()
}

func (s *bubbleSink[T]) AggregateUpdated(snapshot any) {
	s.mu.Lock()
	s.aggregate = snapshot
	s.hasAggregate = true
	s.mu.Unlock()
}

func (s *bubbleSink[T]) SuiteFinished(summary Summary[T]) {
	if s.program == nil || s.done == nil {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}
	s.flushPending()
	s.finished.Store(true)
	s.program.Send(bubbleSuiteFinishedMsg[T]{summary: summary})
	<-s.done
}

func (s *bubbleSink[T]) UserExited() bool {
	return s.exited.Load()
}

func (s *bubbleSink[T]) flushLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.flushPending()
		case <-s.done:
			return
		}
	}
}

func (s *bubbleSink[T]) flushPending() {
	if s.program == nil {
		return
	}
	msg, ok := s.drainPending()
	if !ok {
		return
	}
	s.program.Send(msg)
}

func (s *bubbleSink[T]) drainPending() (bubbleBatchMsg[T], bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 && !s.hasAggregate {
		return bubbleBatchMsg[T]{}, false
	}
	msg := bubbleBatchMsg[T]{
		events:       append([]bubblePendingEvent[T](nil), s.events...),
		aggregate:    s.aggregate,
		hasAggregate: s.hasAggregate,
	}
	s.events = s.events[:0]
	s.aggregate = nil
	s.hasAggregate = false
	return msg, true
}

type bubbleCaseStartedMsg struct {
	event WorkerCaseEvent
}

type bubbleCaseFinishedMsg[T any] struct {
	event WorkerCaseResult[T]
}

type bubbleSuiteFinishedMsg[T any] struct {
	summary Summary[T]
}

type bubbleAggregateUpdatedMsg struct {
	snapshot any
}

type bubbleBatchMsg[T any] struct {
	events       []bubblePendingEvent[T]
	aggregate    any
	hasAggregate bool
}

type bubblePendingEvent[T any] struct {
	started  *WorkerCaseEvent
	finished *WorkerCaseResult[T]
}

type bubbleTickMsg struct{}

type bubbleModel[T any] struct {
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
	workers      []workerSlot
	recent       []string
	recentFilter RecentFilter[T]
	followRecent bool
	viewport     viewport.Model
}

func newBubbleModel[T any](e SuiteEvent, recentFilter RecentFilter[T]) bubbleModel[T] {
	model := bubbleModel[T]{
		name:         e.Name,
		total:        e.Total,
		parallel:     e.Parallel,
		startedAt:    time.Now(),
		workers:      make([]workerSlot, e.Parallel),
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

func (m bubbleModel[T]) Init() tea.Cmd {
	return bubbleTick()
}

func (m bubbleModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case bubbleTickMsg:
		if !m.finished {
			return m, bubbleTick()
		}
	case bubbleCaseStartedMsg:
		m.applyCaseStarted(msg.event)
	case bubbleCaseFinishedMsg[T]:
		if m.applyCaseFinished(msg.event) {
			m.refreshRecent()
		}
	case bubbleBatchMsg[T]:
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
			m.aggregate = formatAggregateTable(msg.aggregate, m.aggregateWidth())
		}
	case bubbleSuiteFinishedMsg[T]:
		m.completed = msg.summary.Total
		m.done = msg.summary.Done
		m.errors = msg.summary.Errors
		m.skipped = msg.summary.Skipped
		m.aggregate = formatAggregateTable(msg.summary.Aggregated, m.aggregateWidth())
		m.finished = true
		m.refreshRecent()
	case bubbleAggregateUpdatedMsg:
		m.aggregate = formatAggregateTable(msg.snapshot, m.aggregateWidth())
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

func (m *bubbleModel[T]) applyCaseStarted(event WorkerCaseEvent) {
	if event.WorkerID >= 0 && event.WorkerID < len(m.workers) {
		m.workers[event.WorkerID] = workerSlot{Name: event.Case.Name, StartedAt: time.Now()}
	}
}

func (m *bubbleModel[T]) applyCaseFinished(event WorkerCaseResult[T]) bool {
	result := event.Result
	if event.WorkerID >= 0 && event.WorkerID < len(m.workers) {
		current := m.workers[event.WorkerID]
		if current.Name == result.Case.Name {
			m.workers[event.WorkerID] = workerSlot{Name: result.Case.Name, StartedAt: result.StartedAt, FinishedAt: result.FinishedAt}
		}
	}
	m.totalCaseMS += result.Duration
	m.completed++
	switch result.State {
	case StateDone:
		m.done++
	case StateSkip:
		m.skipped++
	case StateError:
		m.errors++
	case "":
		m.done++
	default:
		m.errors++
	}
	if m.recentFilter == nil || m.recentFilter(result) {
		m.recent = append(m.recent, m.resultLine(result))
		return true
	}
	return false
}

func (m bubbleModel[T]) View() tea.View {
	m.configureViewport()

	aggregate := ""
	if m.aggregate != "" {
		aggregate = borderedBlock("", m.aggregate, m.width)
	}

	recent := borderedViewportBlock(m.viewport.View(), m.viewport, m.width, m.viewport.Height())

	footer := bubbleMutedStyle.Render("scroll arrows/j/k/pageup/pagedown  q/ctrl+c exits after finish")
	if !m.finished {
		footer = bubbleMutedStyle.Render("scroll arrows/j/k/pageup/pagedown  q/ctrl+c exits")
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		m.progressBar(),
		aggregate,
		"",
		m.workerView(),
		"",
		recent,
		footer,
	)
	view := tea.NewView(content)
	view.AltScreen = true
	// Let the terminal own mouse drag selection so users can copy text from the TUI.
	view.MouseMode = tea.MouseModeNone
	return view
}

func (m *bubbleModel[T]) configureViewport() {
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

func (m bubbleModel[T]) fixedRows() int {
	rows := 7 + m.workerRows()
	if m.aggregate != "" {
		rows += lineCountCLI(borderedBlock("", m.aggregate, m.width))
	}
	return rows
}

const (
	workerBaseBarWidth = 18
	workerCardHeight   = 5
	workerGridGap      = 2
	workerMinCellWidth = workerBaseBarWidth + 12
)

func (m bubbleModel[T]) workerLimit() int {
	limit := len(m.workers)
	if limit < 1 {
		limit = 1
	}
	return limit
}

func (m bubbleModel[T]) workerRows() int {
	limit := m.workerLimit()
	cols := m.workerColumns()
	cardRows := (limit + cols - 1) / cols
	return cardRows * workerCardHeight
}

func (m bubbleModel[T]) workerColumns() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	cols := (width + workerGridGap) / (workerMinCellWidth + workerGridGap)
	if cols < 1 {
		cols = 1
	}
	limit := m.workerLimit()
	if cols > limit {
		cols = limit
	}
	return cols
}

func (m bubbleModel[T]) workerCellWidth() int {
	cols := m.workerColumns()
	if cols < 1 {
		return workerMinCellWidth
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	available := width - (cols-1)*workerGridGap
	if available < cols*workerMinCellWidth {
		return workerMinCellWidth
	}
	return available / cols
}

func (m bubbleModel[T]) progressBar() string {
	prefix := bubbleTitleStyle.Render(m.name) + "  "
	suffix := fmt.Sprintf("  %3d%%  %d/%d  elapsed %s  eta %s", m.percentComplete(), m.completed, m.total, formatDuration(time.Since(m.startedAt)), m.suiteETA())
	width := m.width - lipgloss.Width(prefix) - lipgloss.Width(suffix)
	if width < 10 {
		width = 10
	}

	filled := 0
	if m.total > 0 {
		filled = m.completed * width / m.total
	}
	if filled > width {
		filled = width
	}
	empty := width - filled
	bar := bubbleMeterDoneStyle.Render(strings.Repeat("█", filled)) + bubbleMeterTodoStyle.Render(strings.Repeat("░", empty))
	return prefix + bar + suffix
}

func (m bubbleModel[T]) suiteETA() string {
	elapsed := time.Since(m.startedAt)
	if m.completed > 0 && m.completed < m.total {
		perCase := elapsed / time.Duration(m.completed)
		return formatDuration(perCase * time.Duration(m.total-m.completed))
	}
	if m.completed >= m.total {
		return "done"
	}
	return "calculating"
}

func (m bubbleModel[T]) percentComplete() int {
	if m.total <= 0 {
		return 0
	}
	return m.completed * 100 / m.total
}

func (m bubbleModel[T]) aggregateWidth() int {
	if m.width <= 2 {
		return 78
	}
	return m.width - 2
}

func (m bubbleModel[T]) workerView() string {
	var lines []string
	limit := m.workerLimit()
	cols := m.workerColumns()
	cardRows := (limit + cols - 1) / cols
	cellWidth := m.workerCellWidth()
	for row := 0; row < cardRows; row++ {
		var cells []string
		count := cols
		if remaining := limit - row*cols; remaining < count {
			count = remaining
		}
		for col := 0; col < count; col++ {
			index := row*cols + col
			if index >= limit || index >= len(m.workers) {
				continue
			}
			cells = append(cells, lipgloss.NewStyle().Width(cellWidth).Render(m.workerCell(index, cellWidth)))
		}
		lines = append(lines, strings.Split(joinWorkerCards(cells), "\n")...)
	}
	return strings.Join(lines, "\n")
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

func (m bubbleModel[T]) workerCell(index int, width int) string {
	slot := m.workers[index]
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
	estimate := m.estimatedCaseDuration()
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
	detail := "elapsed " + formatDuration(elapsed)
	if estimate > 0 {
		detail += "  eta " + formatDuration(eta)
	} else {
		detail += "  eta ..."
	}
	return workerCard(title, slot.Name, bar, detail, width)
}

func (m bubbleModel[T]) estimatedCaseDuration() time.Duration {
	if m.completed == 0 || m.totalCaseMS <= 0 {
		return 0
	}
	return time.Duration(m.totalCaseMS/int64(m.completed)) * time.Millisecond
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
	return bubbleMeterDoneStyle.Render(strings.Repeat("█", filled)) + bubbleMeterTodoStyle.Render(strings.Repeat("░", width-filled))
}

func workerCard(title string, caseLine string, progressLine string, timingLine string, width int) string {
	if width < 22 {
		width = 22
	}
	innerWidth := width - 2
	title = truncateCLI(title, innerWidth)
	topFill := innerWidth - lipgloss.Width(title)
	if topFill < 0 {
		topFill = 0
	}

	top := bubbleMutedStyle.Render("╭" + title + strings.Repeat("─", topFill) + "╮")
	caseRow := bubbleMutedStyle.Render("│") + padRightCLI(truncateCLI(caseLine, innerWidth), innerWidth) + bubbleMutedStyle.Render("│")
	middle := bubbleMutedStyle.Render("│") + padRightCLI(progressLine, innerWidth) + bubbleMutedStyle.Render("│")
	bottom := bubbleMutedStyle.Render("│") + padRightCLI(truncateCLI(timingLine, innerWidth), innerWidth) + bubbleMutedStyle.Render("│")
	foot := bubbleMutedStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")
	return strings.Join([]string{top, caseRow, middle, bottom, foot}, "\n")
}

func borderedBlock(title string, body string, width int) string {
	if body == "" {
		return ""
	}
	if width < 20 {
		width = 20
	}
	innerWidth := width - 2
	title = truncateCLI(title, innerWidth)
	topFill := innerWidth - lipgloss.Width(title)
	if topFill < 0 {
		topFill = 0
	}

	top := bubbleMutedStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	if title != "" {
		top = bubbleMutedStyle.Render("╭") + bubbleSectionStyle.Render(title) + bubbleMutedStyle.Render(strings.Repeat("─", topFill)+"╮")
	}
	lines := []string{top}
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, bubbleMutedStyle.Render("│")+padRightCLI(truncateCLI(line, innerWidth), innerWidth)+bubbleMutedStyle.Render("│"))
	}
	lines = append(lines, bubbleMutedStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(lines, "\n")
}

func borderedViewportBlock(body string, view viewport.Model, width int, height int) string {
	if width < 20 {
		width = 20
	}
	if height < 1 {
		height = 1
	}
	innerWidth := width - 2
	contentWidth := innerWidth - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	bodyLines := strings.Split(body, "\n")

	lines := []string{bubbleMutedStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")}
	for i := 0; i < height; i++ {
		line := ""
		if i < len(bodyLines) {
			line = bodyLines[i]
		}
		lines = append(
			lines,
			bubbleMutedStyle.Render("│")+
				padRightCLI(truncateCLI(line, contentWidth), contentWidth)+
				recentScrollbarCell(view, i, height)+
				bubbleMutedStyle.Render("│"),
		)
	}
	lines = append(lines, bubbleMutedStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(lines, "\n")
}

func recentScrollbarCell(view viewport.Model, row int, height int) string {
	total := view.TotalLineCount()
	visible := view.VisibleLineCount()
	if height <= 0 || total <= visible || visible <= 0 {
		return bubbleMutedStyle.Render(" ")
	}

	thumbHeight := visible * height / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	if thumbHeight > height {
		thumbHeight = height
	}

	maxTop := height - thumbHeight
	thumbTop := 0
	if maxTop > 0 {
		thumbTop = int(view.ScrollPercent()*float64(maxTop) + 0.5)
	}
	if row >= thumbTop && row < thumbTop+thumbHeight {
		return bubbleMeterDoneStyle.Render("█")
	}
	return bubbleMutedStyle.Render("│")
}

func truncateCLI(value string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		for _, r := range value {
			return string(r)
		}
		return ""
	}
	var out strings.Builder
	for _, r := range value {
		next := out.String() + string(r)
		if lipgloss.Width(next)+1 > width {
			break
		}
		out.WriteRune(r)
	}
	return out.String() + "…"
}

func padRightCLI(value string, width int) string {
	for lipgloss.Width(value) < width {
		value += " "
	}
	return value
}

func lineCountCLI(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}

func (m *bubbleModel[T]) refreshRecent() {
	m.viewport.SetContent(strings.Join(m.recent, "\n"))
	if m.followRecent {
		m.viewport.GotoBottom()
	}
}

func (m bubbleModel[T]) resultLine(r CaseResult[T]) string {
	if r.State == "" {
		line := fmt.Sprintf("%s (%dms)", r.Case.Name, r.Duration)
		if r.Message != "" {
			line += ": " + r.Message
		}
		return line
	}
	state := string(r.State)
	switch r.State {
	case StateDone:
		state = bubblePassStyle.Render(state)
	case StateError:
		state = bubbleErrorStyle.Render(state)
	default:
		if r.State != StateSkip {
			state = bubbleFailStyle.Render(state)
		}
	}

	line := fmt.Sprintf("[%s] %s (%dms)", state, r.Case.Name, r.Duration)
	if r.Message != "" {
		line += ": " + r.Message
	}
	return line
}

func bubbleTick() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg { return bubbleTickMsg{} })
}

var (
	bubbleTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Blue)
	bubbleSectionStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Yellow)
	bubbleMutedStyle     = lipgloss.NewStyle().Faint(true)
	bubblePassStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green)
	bubbleFailStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Red)
	bubbleErrorStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Yellow)
	bubbleMeterDoneStyle = lipgloss.NewStyle().Foreground(lipgloss.Green)
	bubbleMeterTodoStyle = lipgloss.NewStyle().Faint(true)
)
