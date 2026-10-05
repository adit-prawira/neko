package molecules

import "fmt"

func ResultRow(id string, score float32) string {
	return fmt.Sprintf("%s\t%.4f\n", id, score)
}
