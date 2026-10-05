package tui

import (
	"github.com/adit-prawira/neko/internal/tui/state"
	"github.com/adit-prawira/neko/internal/tui/templates"
)

func Render(m state.Model) string {
	if len(m.Collections) == 0 {
		return templates.Empty()
	}

	return templates.Dashboard(m)
}
