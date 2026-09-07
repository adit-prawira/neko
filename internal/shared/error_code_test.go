package shared

import "testing"

func TestHairballErrorCodeRoundTrip(t *testing.T) {
	cases := []struct {
		code   HairballErrorCode
		intVal int
		strVal string
		http   int
	}{
		{HairballNotFound, 1, "HAIRBALL_NOT_FOUND", 404},
		{HairballAlreadyExists, 2, "HAIRBALL_ALREADY_EXISTS", 409},
		{HairballDimMismatch, 3, "HAIRBALL_DIM_MISMATCH", 400},
		{HairballDimTooLarge, 4, "HAIRBALL_DIM_TOO_LARGE", 400},
		{HairballDimTooSmall, 5, "HAIRBALL_DIM_TOO_SMALL", 400},
		{HairballInvalidName, 6, "HAIRBALL_INVALID_NAME", 400},
		{HairballIOError, 7, "HAIRBALL_IO_ERROR", 500},
		{HairballSerializeError, 8, "HAIRBALL_SERIALIZE_ERROR", 500},
		{HairballCorruptedSegment, 9, "HAIRBALL_CORRUPTED_SEGMENT", 500},
		{HairballInternalError, 10, "HAIRBALL_INTERNAL_ERROR", 500},
		{HairballInvalidMetric, 11, "HAIRBALL_INVALID_METRIC", 400},
	}

	for _, tc := range cases {
		t.Run("given "+tc.strVal+", then Int/String/Http map match", func(t *testing.T) {
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
