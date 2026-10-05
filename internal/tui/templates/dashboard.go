package templates

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/adit-prawira/neko/internal/tui/atoms"
	"github.com/adit-prawira/neko/internal/tui/organisms"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func Dashboard(m state.Model) string {
	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		atoms.Panel.Width(30).Render(organisms.CollectionsPanel(m)),
		atoms.Panel.Width(40).Render(organisms.DetailsPanel(m)),
	)

	bottom := lipgloss.JoinHorizontal(
		lipgloss.Top,
		atoms.Panel.Width(30).Render(organisms.SearchPanel(m)),
		atoms.Panel.Width(40).Render(organisms.ResultsPanel(m)),
	)

	main := lipgloss.JoinVertical(
		lipgloss.Left,
		top,
		bottom,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		atoms.Title.Render(" neko tui "),
		main,
		atoms.Status.Render(fmt.Sprintf("latency: %s | %s", m.LastLatency, m.StatusMessage)),
	)
}
