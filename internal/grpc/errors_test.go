package grpcserver

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/adit-prawira/neko/internal/ffi"
)

func TestHairballToGRPCStatus(t *testing.T) {
	t.Run("given nil error, then returns nil", func(t *testing.T) {
		if got := HairballToGRPCStatus(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("given HairballError code 1 (NotFound), then status NotFound", func(t *testing.T) {
		err := &ffi.HairballError{FunctionName: "neko_test", Code: 1}
		got := HairballToGRPCStatus(err)
		if status.Code(got) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(got), codes.NotFound)
		}
	})
	t.Run("given HairballError code 2 (AlreadyExists), then status AlreadyExists", func(t *testing.T) {
		err := &ffi.HairballError{FunctionName: "neko_test", Code: 2}
		got := HairballToGRPCStatus(err)
		if status.Code(got) != codes.AlreadyExists {
			t.Errorf("got code %v, want %v", status.Code(got), codes.AlreadyExists)
		}
	})
	t.Run("given HairballError code 6 (InvalidName), then status InvalidArgument", func(t *testing.T) {
		err := &ffi.HairballError{FunctionName: "neko_test", Code: 6}
		got := HairballToGRPCStatus(err)
		if status.Code(got) != codes.InvalidArgument {
			t.Errorf("got code %v, want %v", status.Code(got), codes.InvalidArgument)
		}
	})
	t.Run("given HairballError with unmapped code, then status Internal", func(t *testing.T) {
		err := &ffi.HairballError{FunctionName: "neko_test", Code: 99}
		got := HairballToGRPCStatus(err)
		if status.Code(got) != codes.Internal {
			t.Errorf("got code %v, want %v", status.Code(got), codes.Internal)
		}
	})
	t.Run("given non-HairballError, then status Internal", func(t *testing.T) {
		err := errors.New("plain error")
		got := HairballToGRPCStatus(err)
		if status.Code(got) != codes.Internal {
			t.Errorf("got code %v, want %v", status.Code(got), codes.Internal)
		}
	})
	t.Run("given wrapped HairballError, then errors.As unwraps to NotFound", func(t *testing.T) {
		wrapped := fmt.Errorf("neko_create failed: %w", &ffi.HairballError{FunctionName: "neko_create", Code: 1})
		got := HairballToGRPCStatus(wrapped)
		if status.Code(got) != codes.NotFound {
			t.Errorf("got code %v, want %v", status.Code(got), codes.NotFound)
		}
	})
}
