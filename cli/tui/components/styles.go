package components

import "charm.land/lipgloss/v2"

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Blue)
	sectionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Yellow)
	mutedStyle       = lipgloss.NewStyle().Faint(true)
	passStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green)
	failStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Red)
	errorStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Yellow)
	meterDoneStyle   = lipgloss.NewStyle().Foreground(lipgloss.Green)
	meterTodoStyle   = lipgloss.NewStyle().Faint(true)
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Black).Background(lipgloss.Green)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.White).Background(lipgloss.Color("238"))
)
