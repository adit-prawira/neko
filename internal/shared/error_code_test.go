package shared

import (
	"testing"

	"google.golang.org/grpc/codes"
)

func TestHairballErrorCodeRoundTrip(t *testing.T) {
	cases := []struct {
		code   HairballErrorCode
		intVal int
		strVal string
		http   int
		grpc   codes.Code
	}{
		{HairballNotFound, 1, "HAIRBALL_NOT_FOUND", 404, codes.NotFound},
		{HairballAlreadyExists, 2, "HAIRBALL_ALREADY_EXISTS", 409, codes.AlreadyExists},
		{HairballDimMismatch, 3, "HAIRBALL_DIM_MISMATCH", 400, codes.InvalidArgument},
		{HairballDimTooLarge, 4, "HAIRBALL_DIM_TOO_LARGE", 400, codes.InvalidArgument},
		{HairballDimTooSmall, 5, "HAIRBALL_DIM_TOO_SMALL", 400, codes.InvalidArgument},
		{HairballInvalidName, 6, "HAIRBALL_INVALID_NAME", 400, codes.InvalidArgument},
		{HairballIOError, 7, "HAIRBALL_IO_ERROR", 500, codes.Internal},
		{HairballSerializeError, 8, "HAIRBALL_SERIALIZE_ERROR", 500, codes.Internal},
		{HairballCorruptedSegment, 9, "HAIRBALL_CORRUPTED_SEGMENT", 500, codes.Internal},
		{HairballInternalError, 10, "HAIRBALL_INTERNAL_ERROR", 500, codes.Internal},
		{HairballInvalidMetric, 11, "HAIRBALL_INVALID_METRIC", 400, codes.InvalidArgument},
	}

	for _, tc := range cases {
		t.Run("given "+tc.strVal+", then Int/String/Http/GRPC maps match", func(t *testing.T) {
			if got := tc.code.Int(); got != tc.intVal {
				t.Errorf("Int(): got %d, want %d", got, tc.intVal)
			}
			if got := tc.code.String(); got != tc.strVal {
				t.Errorf("String(): got %q, want %q", got, tc.strVal)
			}
			if got := HairballCodeToString[tc.intVal]; got != tc.strVal {
				t.Errorf("HairballCodeToString[%d]: got %q, want %q", tc.intVal, got, tc.strVal)
			}
			if got := HairballCodeToHTTP[tc.intVal]; got != tc.http {
				t.Errorf("HairballCodeToHTTP[%d]: got %d, want %d", tc.intVal, got, tc.http)
			}
			if got := HairballToGRPC[tc.intVal]; got != tc.grpc {
				t.Errorf("HairballToGRPC[%d]: got %v, want %v", tc.intVal, got, tc.grpc)
			}
		})
	}
}

func TestHairballErrorCodeIntUnknownReturnsZero(t *testing.T) {
	t.Run("given unknown code, then Int returns 0", func(t *testing.T) {
		unknown := HairballErrorCode("HAIRBALL_UNKNOWN")
		if got := unknown.Int(); got != 0 {
			t.Errorf("expected 0 for unknown code, got %d", got)
		}
	})
}
