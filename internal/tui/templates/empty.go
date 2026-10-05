package templates

import (
	"charm.land/lipgloss/v2"
	"github.com/adit-prawira/neko/internal/tui/atoms"
)

func Empty() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		atoms.Title.Render(" neko tui "),
		atoms.Panel.Render(" No collections yet.\n\nUse `neko create <name> --dim <n>` to get started."),
		atoms.Status.Render("q to quit"),
	)
}
