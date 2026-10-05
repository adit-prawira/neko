package molecules

import "github.com/adit-prawira/neko/internal/tui/atoms"

func CollectionRow(name string, selected bool) string {
	if selected {
		return atoms.Selected.Render(name)
	}

	return name
}
