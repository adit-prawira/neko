package shared

import (
	"fmt"
	"os"
	"unsafe"
)

func ParseVectorFile(path string) ([]float32, error) {
	data, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("cannot read vector file: %w", err)
	}

	if len(data)%4 != 0 {
		return nil, fmt.Errorf("file size %d is not a multiple of 4 bytes", len(data))
	}

	return unsafe.Slice((*float32)(unsafe.Pointer(&data[0])), len(data)/4), nil
}
