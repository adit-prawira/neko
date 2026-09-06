package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
)

func TestWriteFFIError(t *testing.T) {
	t.Run("given NotFound code, then returns 404 with HAIRBALL_NOT_FOUND", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		WriteFFIError(recorder, &ffi.HairballError{FunctionName: "neko_drop", Code: 1})

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_NOT_FOUND" {
			t.Errorf("expected HAIRBALL_NOT_FOUND, got %s", code)
		}
	})

	t.Run("given AlreadyExists code, then returns 409 with HAIRBALL_ALREADY_EXISTS", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		WriteFFIError(recorder, &ffi.HairballError{FunctionName: "neko_create", Code: 2})

		if recorder.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_ALREADY_EXISTS" {
			t.Errorf("expected HAIRBALL_ALREADY_EXISTS, got %s", code)
		}
	})

	t.Run("given DimMismatch code, then returns 400 with HAIRBALL_DIM_MISMATCH", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		WriteFFIError(recorder, &ffi.HairballError{FunctionName: "neko_insert", Code: 3})

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_DIM_MISMATCH" {
			t.Errorf("expected HAIRBALL_DIM_MISMATCH, got %s", code)
		}
	})

	t.Run("given InvalidName code, then returns 400 with HAIRBALL_INVALID_NAME", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		WriteFFIError(recorder, &ffi.HairballError{FunctionName: "neko_create", Code: 6})

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INVALID_NAME" {
			t.Errorf("expected HAIRBALL_INVALID_NAME, got %s", code)
		}
	})

	t.Run("given IOError code, then returns 500 with HAIRBALL_IO_ERROR", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		WriteFFIError(recorder, &ffi.HairballError{FunctionName: "neko_stats", Code: 7})

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_IO_ERROR" {
			t.Errorf("expected HAIRBALL_IO_ERROR, got %s", code)
		}
	})

	t.Run("given unknown code, then returns 500 with HAIRBALL_INTERNAL_ERROR", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		WriteFFIError(recorder, &ffi.HairballError{FunctionName: "neko_unknown", Code: 99})

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INTERNAL_ERROR" {
			t.Errorf("expected HAIRBALL_INTERNAL_ERROR, got %s", code)
		}
	})

	t.Run("given non-hairball error, then returns 500 with HAIRBALL_INTERNAL_ERROR", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		WriteFFIError(recorder, errors.New("something broke"))

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "HAIRBALL_INTERNAL_ERROR" {
			t.Errorf("expected HAIRBALL_INTERNAL_ERROR, got %s", code)
		}
	})
}
