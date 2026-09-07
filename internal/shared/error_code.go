package shared

import "net/http"

type HairballErrorCode string

const (
	HairballNotFound         HairballErrorCode = "HAIRBALL_NOT_FOUND"
	HairballAlreadyExists    HairballErrorCode = "HAIRBALL_ALREADY_EXISTS"
	HairballDimMismatch      HairballErrorCode = "HAIRBALL_DIM_MISMATCH"
	HairballDimTooLarge      HairballErrorCode = "HAIRBALL_DIM_TOO_LARGE"
	HairballDimTooSmall      HairballErrorCode = "HAIRBALL_DIM_TOO_SMALL"
	HairballInvalidName      HairballErrorCode = "HAIRBALL_INVALID_NAME"
	HairballIOError          HairballErrorCode = "HAIRBALL_IO_ERROR"
	HairballSerializeError   HairballErrorCode = "HAIRBALL_SERIALIZE_ERROR"
	HairballCorruptedSegment HairballErrorCode = "HAIRBALL_CORRUPTED_SEGMENT"
	HairballInternalError    HairballErrorCode = "HAIRBALL_INTERNAL_ERROR"
	HairballInvalidMetric    HairballErrorCode = "HAIRBALL_INVALID_METRIC"
)

func (hec HairballErrorCode) String() string {
	return string(hec)
}

func (hec HairballErrorCode) Int() int {
	switch hec {
	case HairballNotFound:
		return 1
	case HairballAlreadyExists:
		return 2
	case HairballDimMismatch:
		return 3
	case HairballDimTooLarge:
		return 4
	case HairballDimTooSmall:
		return 5
	case HairballInvalidName:
		return 6
	case HairballIOError:
		return 7
	case HairballSerializeError:
		return 8
	case HairballCorruptedSegment:
		return 9
	case HairballInternalError:
		return 10
	case HairballInvalidMetric:
		return 11
	default:
		return 0
	}
}

var HairballCodeToString = map[int]string{
	1:  "HAIRBALL_NOT_FOUND",
	2:  "HAIRBALL_ALREADY_EXISTS",
	3:  "HAIRBALL_DIM_MISMATCH",
	4:  "HAIRBALL_DIM_TOO_LARGE",
	5:  "HAIRBALL_DIM_TOO_SMALL",
	6:  "HAIRBALL_INVALID_NAME",
	7:  "HAIRBALL_IO_ERROR",
	8:  "HAIRBALL_SERIALIZE_ERROR",
	9:  "HAIRBALL_CORRUPTED_SEGMENT",
	10: "HAIRBALL_INTERNAL_ERROR",
	11: "HAIRBALL_INVALID_METRIC",
}

var HairballCodeToHTTP = map[int]int{
	1:  http.StatusNotFound,
	2:  http.StatusConflict,
	3:  http.StatusBadRequest,
	4:  http.StatusBadRequest,
	5:  http.StatusBadRequest,
	6:  http.StatusBadRequest,
	7:  http.StatusInternalServerError,
	8:  http.StatusInternalServerError,
	9:  http.StatusInternalServerError,
	10: http.StatusInternalServerError,
	11: http.StatusBadRequest,
}
