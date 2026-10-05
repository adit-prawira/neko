package organisms

import (
	"fmt"
	"strings"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/shared"
	"github.com/adit-prawira/neko/internal/tui/state"
)

func DetailsPanel(m state.Model) string {
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
