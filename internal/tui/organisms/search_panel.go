package organisms

import (
	"strings"

	"github.com/adit-prawira/neko/internal/tui/atoms"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func SearchPanel(m state.Model) string {
	var builder strings.Builder
	builder.WriteString("Search\n\n")
	if m.IsSearchPanelFocused() {
		builder.WriteString(atoms.Focused.Render(m.SearchInput))
		builder.WriteString("▌")
	} else {
		builder.WriteString(m.SearchInput)
	}

	builder.WriteString("\n\n")
	builder.WriteString(atoms.Status.Render("press / to focus, enter to search"))
	return builder.String()
}
