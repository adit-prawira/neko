package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#874BFD")).Padding(1)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3A3A3"))
)

func renderEmpty() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(" neko tui "),
		panelStyle.Render(" No collections yet.\n\nUse `neko create <name> --dim <n>` to get started."),
		statusStyle.Render("q to quit"),
	)
}

func renderView(m Model) string {
	if len(m.Collections) == 0 {
		return renderEmpty()
	}

	var builder strings.Builder
	builder.WriteString("Collections\n\n")
	for _, collection := range m.Collections {
		builder.WriteString(collection.Name)
		builder.WriteString("\n")
	}

	main := panelStyle.Render(builder.String())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(" neko tui "),
		main,
		statusStyle.Render("q to quit"),
	)
}
