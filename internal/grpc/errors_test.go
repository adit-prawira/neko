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

	cases := []struct {
		code int
		want codes.Code
	}{
		{1, codes.NotFound},
		{2, codes.AlreadyExists},
		{3, codes.InvalidArgument},
		{4, codes.InvalidArgument},
		{5, codes.InvalidArgument},
		{6, codes.InvalidArgument},
		{7, codes.Internal},
		{8, codes.Internal},
		{9, codes.Internal},
		{10, codes.Internal},
		{11, codes.InvalidArgument},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("given HairballError code %d, then status %v", tc.code, tc.want), func(t *testing.T) {
			err := &ffi.HairballError{FunctionName: "neko_test", Code: tc.code}
			got := HairballToGRPCStatus(err)
			if status.Code(got) != tc.want {
				t.Errorf("got code %v, want %v", status.Code(got), tc.want)
			}
		})
	}

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
