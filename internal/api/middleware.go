package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/shared"
)

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
		code := shared.HairballCodeToString[hairballError.Code]
		if code == "" {
			code = shared.HairballInternalError.String()
		}

		status := shared.HairballCodeToHTTP[hairballError.Code]
		if status == 0 {
			status = http.StatusInternalServerError
		}

		WriteHairball(rw, status, code, err.Error())
		return
	}
	WriteHairball(rw, http.StatusInternalServerError, shared.HairballInternalError.String(), err.Error())
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
				WriteHairball(rw, http.StatusInternalServerError, shared.HairballInternalError.String(), "internal server error")
			}
		}()
		next.ServeHTTP(rw, r)
	})
}
