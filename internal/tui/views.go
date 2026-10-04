package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/shared"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#874BFD")).Padding(1)
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3A3A3"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#874BFD"))
	focusedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4"))
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
	search := renderSearch(m)
	results := renderResults(m)

	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		panelStyle.Width(30).Render(collections),
		panelStyle.Width(40).Render(details),
	)

	bottomRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		panelStyle.Width(30).Render(search),
		panelStyle.Width(40).Render(results),
	)

	main := lipgloss.JoinVertical(
		lipgloss.Left,
		topRow,
		bottomRow,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(" neko tui "),
		main,
		statusStyle.Render(fmt.Sprintf("latency: %s | %s", m.LastLatency, m.StatusMessage)),
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

func renderSearch(m Model) string {
	var builder strings.Builder
	builder.WriteString("Search\n\n")
	if m.isSearchPanelFocused() {
		builder.WriteString(focusedStyle.Render(m.SearchInput))
		builder.WriteString("▌")
	} else {
		builder.WriteString(m.SearchInput)
	}

	builder.WriteString("\n\n")
	builder.WriteString(statusStyle.Render("press / to focus, enter to search"))
	return builder.String()
}

func renderResults(m Model) string {
	var builder strings.Builder
	builder.WriteString("Results\n\n")

	hasSearchResults := len(m.SearchResults) > 0
	if !hasSearchResults {
		builder.WriteString(statusStyle.Render("no search yet"))
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("found %d results\n\n", len(m.SearchResults)))
	for _, result := range m.SearchResults {
		builder.WriteString(fmt.Sprintf("%s\t%.4f\n", result.ID, result.Score))
	}
	return builder.String()
}

func metricLabel(metric uint8) string {
	if label, ok := ffi.MetricNames[metric]; ok {
		return label
	}

	return "unknown"
}
