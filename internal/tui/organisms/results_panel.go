package organisms

import (
	"fmt"
	"strings"

	"github.com/adit-prawira/neko/internal/tui/atoms"
	"github.com/adit-prawira/neko/internal/tui/molecules"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func ResultsPanel(m state.Model) string {
	var builder strings.Builder
	builder.WriteString("Results\n\n")

	hasSearchResults := len(m.SearchResults) > 0
	if !hasSearchResults {
		builder.WriteString(atoms.Status.Render("no search yet"))
		return builder.String()
	}

	builder.WriteString(fmt.Sprintf("found %d results\n\n", len(m.SearchResults)))
	for _, result := range m.SearchResults {
		builder.WriteString(molecules.ResultRow(result.ID, result.Score))
	}
	return builder.String()
}
