package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adit-prawira/neko/internal/shared"
)

func ParseQuery(input string) ([]float32, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty query")
	}

	if strings.ContainsAny(input, "/\\") {
		return shared.ParseVectorFile(input)
	}

	input = strings.ReplaceAll(input, ",", " ")
	parts := strings.Fields(input)

	floats := make([]float32, len(parts))
	for i, part := range parts {
		value, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", part, err)
		}

		floats[i] = float32(value)
	}

	return floats, nil
}
