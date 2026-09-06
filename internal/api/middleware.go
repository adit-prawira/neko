package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/adit-prawira/neko/internal/ffi"
)

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

var hairballCodeToString = map[int]string{
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

var hairballCodeToHTTP = map[int]int{
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

type HairballDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIError struct {
	Error HairballDetail `json:"error"`
}

func WriteJSON(rw http.ResponseWriter, status int, payload any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)

	if payload != nil {
		json.NewEncoder(rw).Encode(payload)
	}
}

func WriteHairball(rw http.ResponseWriter, status int, code string, message string) {
	WriteJSON(rw, status, APIError{
		Error: HairballDetail{
			Code:    code,
			Message: message,
		},
	})
}

func WriteFFIError(rw http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	var hairballError *ffi.HairballError
	if errors.As(err, &hairballError) {
		code := hairballCodeToString[hairballError.Code]
		if code == "" {
			code = HairballInternalError.String()
		}

		status := hairballCodeToHTTP[hairballError.Code]
		if status == 0 {
			status = http.StatusInternalServerError
		}

		WriteHairball(rw, status, code, err.Error())
		return
	}
	WriteHairball(rw, http.StatusInternalServerError, HairballInternalError.String(), err.Error())
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		rw.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(rw, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func RequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: rw,
			status:         http.StatusOK,
		}
		next.ServeHTTP(recorder, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration", time.Since(start).String())
	})
}

func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic in handler", "error", recovered, "stack", string(debug.Stack()))
				WriteHairball(rw, http.StatusInternalServerError, "HAIRBALL_INTERNAL_ERROR", "internal server error")
			}
		}()
		next.ServeHTTP(rw, r)
	})
}
