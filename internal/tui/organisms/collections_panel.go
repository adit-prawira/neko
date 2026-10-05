package organisms

import (
	"strings"

	"github.com/adit-prawira/neko/internal/tui/molecules"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func CollectionsPanel(m state.Model) string {
	var builder strings.Builder
	builder.WriteString("Collections\n\n")
	for i, collection := range m.Collections {
		builder.WriteString(molecules.CollectionRow(collection.Name, i == m.SelectedIndex))
		builder.WriteString("\n")
	}
	return builder.String()
}
