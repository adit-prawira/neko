package shared

import "fmt"

func FormatBytes(n uint64) string {
	const k = uint64(1024)
	switch {
	case n < k:
		return fmt.Sprintf("%dB", n)
	case n < k*k:
		return fmt.Sprintf("%dKB", n/k)
	case n < k*k*k:
		return fmt.Sprintf("%dMB", n/(k*k))
	default:
		return fmt.Sprintf("%dGB", n/(k*k*k))
	}
}
