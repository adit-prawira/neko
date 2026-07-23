package ffi

/*
#cgo LDFLAGS: -L${SRCDIR}/../../engine/target/release -lneko_engine
#include <stdlib.h>
#include "bridge.h"
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

const (
	MetricL2     = 0
	MetricCosine = 1
	MetricDot    = 2
)

type NekoStats struct {
	VectorCount  uint64
	Dim          uint32
	Metric       uint8
	StorageBytes uint64
	IndexType    uint8
}

var MetricNames = map[uint8]string{
	MetricL2:     "l2",
	MetricCosine: "cosine",
	MetricDot:    "dot",
}

var metricCodes = map[string]uint8{
	"l2":     MetricL2,
	"cosine": MetricCosine,
	"dot":    MetricDot,
}

func Version() string {
	code := C.neko_version()

	if code != 0 {
		return fmt.Sprintf("neko v0.1.0 (engine error: %d)", int(code))
	}

	return "neko v0.1.0"
}

func Init(dataDirectory string) error {
	cDataDirectory := C.CString(dataDirectory)

	// free memory when function finished executing
	defer C.free(unsafe.Pointer(cDataDirectory))

	code := C.neko_init(cDataDirectory)
	if code != 0 {
		return fmt.Errorf("engine init failed: error code %d", code)
	}
	return nil
}

func DefaultDataDirectory() string {
	if dir := os.Getenv("NEKO_HOME"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".neko")
	}

	return filepath.Join(home, ".neko")
}

func ShutDown() error {
	code := C.neko_shutdown()
	if code != 0 {
		return fmt.Errorf("engine shutdown failed: error code %d", code)
	}
	return nil
}

func Create(name string, dim uint32, metric uint8, model string) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	var cModel *C.char
	if model != "" {
		cModel = C.CString(model)
		defer C.free(unsafe.Pointer(cModel))
	}

	code := C.neko_create(cName, C.uint32_t(dim), C.uint8_t(metric), cModel)
	if code != 0 {
		return fmt.Errorf("cannot create collection '%s', error code %d", name, int(code))
	}
	return nil
}

func List() ([]string, error) {
	var cNames **C.char
	var cCount C.uint

	code := C.neko_list(&cNames, &cCount)
	if code != 0 {
		return nil, fmt.Errorf("cannot list collections: error code %d", int(code))
	}

	defer C.neko_free_strings(cNames, cCount)

	count := int(cCount)
	names := make([]string, count)
	cStrings := unsafe.Slice(cNames, count)
	for i := range cStrings {
		names[i] = C.GoString(cStrings[i])
	}
	return names, nil
}

func Drop(name string) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	code := C.neko_drop(cName)
	if code != 0 {
		return fmt.Errorf("cannot drop collection '%s': error code %d", name, int(code))
	}

	return nil
}

func Stats(name string) (NekoStats, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var stats C.NekoStats
	code := C.neko_stats(cName, &stats)

	if code != 0 {
		return NekoStats{}, fmt.Errorf("cannot get stats for '%s': error code %d", name, int(code))
	}

	return NekoStats{
		VectorCount:  uint64(stats.vector_count),
		Dim:          uint32(stats.dim),
		Metric:       uint8(stats.metric),
		StorageBytes: uint64(stats.storage_bytes),
		IndexType:    uint8(stats.index_type),
	}, nil
}

func Insert(name, id string, vector []float32, metadata string) error {
	if len(vector) == 0 {
		return fmt.Errorf("vector must not be empty")
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))

	var cMeta *C.char
	if metadata != "" {
		cMeta = C.CString(metadata)
		defer C.free(unsafe.Pointer(cMeta))
	}

	code := C.neko_insert(cName, cId, (*C.float)(&vector[0]), C.uint32_t(len(vector)), cMeta)
	if code != 0 {
		return fmt.Errorf("cannot insert vector '%s' into '%s', error code %d", id, name, int(code))
	}
	return nil
}

func Get(name, id string, dim uint32) ([]float32, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))

	vector := make([]float32, dim)
	code := C.neko_get(cName, cId, (*C.float)(&vector[0]), C.uint32_t(dim))
	if code != 0 {
		return nil, fmt.Errorf("cannot get vector '%s' from '%s': error code %d", id, name, code)
	}

	return vector, nil
}

func ParseMetric(name string) (uint8, error) {
	code, ok := metricCodes[name]
	if !ok {
		return 0, fmt.Errorf("invalid metric '%s': Must be: l2, cosine, or dot", name)
	}

	return code, nil
}
