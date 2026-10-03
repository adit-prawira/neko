package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/shared"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#874BFD")).Padding(1)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3A3A3"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#874BFD"))
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

	collections := renderCollections(m)	
	details := renderDetails(m)

	main := lipgloss.JoinHorizontal(
		lipgloss.Top, 
		panelStyle.Width(30).Render(collections),
		panelStyle.Width(40).Render(details),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(" neko tui "),
		main,
		statusStyle.Render("↑/↓ to select, q to quit"),
	)
}

func renderCollections(m Model) string {
	var builder strings.Builder
	builder.WriteString("Collections\n\n")
	for i, collection := range m.Collections {
		line := collection.Name
		if i == m.SelectedIndex {
			builder.WriteString(selectedStyle.Render(line))
		} else {
			builder.WriteString(line)
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

func renderDetails(m Model) string {
	isValidIndex := m.SelectedIndex >= 0 && m.SelectedIndex < len(m.Collections)
	if !isValidIndex {
		return "Details\n\nNo collection selected"
	}

	selectedCollection := m.Collections[m.SelectedIndex]
	
	var builder strings.Builder
	builder.WriteString("Details\n\n")
	builder.WriteString(fmt.Sprintf("Name: %s\n", selectedCollection.Name))
	builder.WriteString(fmt.Sprintf("Dim: %d\n", selectedCollection.Stats.Dim))
	builder.WriteString(fmt.Sprintf("Metric: %s\n", metricLabel(selectedCollection.Stats.Metric)))
	builder.WriteString(fmt.Sprintf("Vectors: %d\n", selectedCollection.Stats.VectorCount))
	builder.WriteString(fmt.Sprintf("Storage: %s\n", shared.FormatBytes(selectedCollection.Stats.StorageBytes)))
	return builder.String()
}

func metricLabel(metric uint8) string {
	if label, ok := ffi.MetricNames[metric]; ok {
		return label
	}

	return "unknown"
}
