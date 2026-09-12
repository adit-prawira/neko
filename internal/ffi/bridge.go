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

	"github.com/adit-prawira/neko/internal/shared"
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

type NekoSearchResult struct {
	ID    string
	Score float32
}

type HairballError struct {
	FunctionName string
	Code         int
}

func (he *HairballError) Error() string {
	return fmt.Sprintf("'%s': error code %d", he.FunctionName, he.Code)
}

func newHairballError(functionName string, code int) error {
	return &HairballError{
		FunctionName: functionName,
		Code:         code,
	}
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
		return newHairballError("neko_init", int(code))
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
		return newHairballError("neko_shutdown", int(code))
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
		return newHairballError("neko_create", int(code))
	}
	return nil
}

func List() ([]string, error) {
	var cNames **C.char
	var cCount C.uint

	code := C.neko_list(&cNames, &cCount)
	if code != 0 {
		return nil, newHairballError("neko_list", int(code))
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
		return newHairballError("neko_drop", int(code))
	}

	return nil
}

func Stats(name string) (NekoStats, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var stats C.NekoStats
	code := C.neko_stats(cName, &stats)

	if code != 0 {
		return NekoStats{}, newHairballError("neko_stats", int(code))
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
		return newHairballError("neko_insert", shared.HairballDimTooSmall.Int())
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
		return newHairballError("neko_insert", int(code))
	}
	return nil
}

type InputVector struct {
	ID       string
	Vector   []float32
	Metadata string
}

func InsertMany(name string, inputVectors []InputVector) error {
	if len(inputVectors) == 0 {
		return newHairballError("neko_insert_many", shared.HairballDimTooSmall.Int())
	}

	count := len(inputVectors)
	cCount := C.uint32_t(count)
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	totalFloats := 0
	for _, item := range inputVectors {
		totalFloats += len(item.Vector)
	}
	var cVectorBuffer *C.float
	if totalFloats > 0 {
		cVectorBuffer = (*C.float)(C.malloc(C.size_t(totalFloats) * C.sizeof_float))
		defer C.free(unsafe.Pointer(cVectorBuffer))
	}
	cVectorSlice := unsafe.Slice(cVectorBuffer, totalFloats)

	cInputVectors := make([]C.NekoInputVector, count)
	writeOffset := 0
	for index, item := range inputVectors {
		cID := C.CString(item.ID)
		defer C.free(unsafe.Pointer(cID))

		dim := len(item.Vector)
		cDim := C.uint32_t(dim)

		for _, component := range item.Vector {
			cVectorSlice[writeOffset] = C.float(component)
			writeOffset++
		}

		var cVector *C.float
		if dim > 0 {
			cVector = &cVectorSlice[writeOffset-dim]
		}

		var cMetadata *C.char
		if item.Metadata != "" {
			cMetadata = C.CString(item.Metadata)
			defer C.free(unsafe.Pointer(cMetadata))
		}

		cInputVectors[index] = C.NekoInputVector{
			id:       cID,
			vector:   cVector,
			dim:      cDim,
			metadata: cMetadata,
		}
	}

	code := C.neko_insert_many(cName, &cInputVectors[0], cCount)
	if code != 0 {
		return newHairballError("neko_insert_many", int(code))
	}
	return nil
}

func Upsert(name, id string, vector []float32, metadata string) (bool, error) {
	if len(vector) == 0 {
		return false, newHairballError("neko_upsert", shared.HairballDimTooSmall.Int())
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

	var cCreated C.uint8_t
	code := C.neko_upsert(cName, cId, (*C.float)(&vector[0]), C.uint32_t(len(vector)), cMeta, &cCreated)
	if code != 0 {
		return false, newHairballError("neko_upsert", int(code))
	}
	return cCreated == 1, nil
}

func Get(name, id string, dim uint32) ([]float32, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))

	vector := make([]float32, dim)
	code := C.neko_get(cName, cId, (*C.float)(&vector[0]), C.uint32_t(dim))
	if code != 0 {
		return nil, newHairballError("neko_get", int(code))
	}

	return vector, nil
}

func Delete(name, id string) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cId := C.CString(id)
	defer C.free(unsafe.Pointer(cId))

	code := C.neko_delete(cName, cId)
	if code != 0 {
		return newHairballError("neko_delete", int(code))
	}

	return nil
}

func Search(name string, query []float32, topK uint32) ([]NekoSearchResult, error) {
	if len(query) == 0 {
		return nil, newHairballError("neko_search", shared.HairballDimTooSmall.Int())
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var result C.NekoSearchResult
	code := C.neko_search(cName, (*C.float)(&query[0]), C.uint32_t(len(query)), C.uint32_t(topK), &result)
	if code != 0 {
		return nil, newHairballError("neko_search", int(code))
	}
	defer C.neko_free_result(&result)
	total := int(result.total)
	if total == 0 {
		return []NekoSearchResult{}, nil
	}
	results := make([]NekoSearchResult, total)
	ids := unsafe.Slice(result.ids, total)
	scores := unsafe.Slice(result.scores, total)

	for i := range results {
		results[i] = NekoSearchResult{
			ID:    C.GoString(ids[i]),
			Score: float32(scores[i]),
		}
	}
	return results, nil
}

func GetVector(name, id string, dim uint32) ([]float32, string, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	vector := make([]C.float, dim)
	var meta C.NekoMetadata
	code := C.neko_get_vector(cName, cID, (*C.float)(unsafe.Pointer(&vector[0])), C.uint32_t(dim), &meta)
	if code != 0 {
		return nil, "", newHairballError("neko_get_vector", int(code))
	}
	defer C.neko_free_metadata(&meta)

	out := make([]float32, dim)
	for index, value := range vector {
		out[index] = float32(value)
	}
	var metadataString string
	if meta.metadata != nil {
		metadataString = C.GoString(meta.metadata)
	}

	return out, metadataString, nil
}

func ParseMetric(name string) (uint8, error) {
	code, ok := metricCodes[name]
	if !ok {
		return 0, fmt.Errorf("invalid metric '%s': Must be: l2, cosine, or dot", name)
	}

	return code, nil
}
