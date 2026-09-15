package grpcserver

import (
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
)

func TestProtoMetricToFFI(t *testing.T) {
	t.Run("given proto METRIC_L2, then ffi MetricL2", func(t *testing.T) {
		got, err := protoMetricToFFI(nekov1.Metric_METRIC_L2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ffi.MetricL2 {
			t.Errorf("got %d, want ffi.MetricL2 (%d)", got, ffi.MetricL2)
		}
	})
	t.Run("given proto METRIC_COSINE, then ffi MetricCosine", func(t *testing.T) {
		got, err := protoMetricToFFI(nekov1.Metric_METRIC_COSINE)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ffi.MetricCosine {
			t.Errorf("got %d, want ffi.MetricCosine (%d)", got, ffi.MetricCosine)
		}
	})
	t.Run("given proto METRIC_DOT, then ffi MetricDot", func(t *testing.T) {
		got, err := protoMetricToFFI(nekov1.Metric_METRIC_DOT)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ffi.MetricDot {
			t.Errorf("got %d, want ffi.MetricDot (%d)", got, ffi.MetricDot)
		}
	})
	t.Run("given proto METRIC_UNSPECIFIED, then error", func(t *testing.T) {
		_, err := protoMetricToFFI(nekov1.Metric_METRIC_UNSPECIFIED)
		if err == nil {
			t.Fatal("expected error for METRIC_UNSPECIFIED, got nil")
		}
	})
}

func TestFFIMetricToProto(t *testing.T) {
	t.Run("given ffi MetricL2, then proto METRIC_L2", func(t *testing.T) {
		got := ffiMetricToProto(ffi.MetricL2)
		if got != nekov1.Metric_METRIC_L2 {
			t.Errorf("got %v, want %v", got, nekov1.Metric_METRIC_L2)
		}
	})
	t.Run("given ffi MetricCosine, then proto METRIC_COSINE", func(t *testing.T) {
		got := ffiMetricToProto(ffi.MetricCosine)
		if got != nekov1.Metric_METRIC_COSINE {
			t.Errorf("got %v, want %v", got, nekov1.Metric_METRIC_COSINE)
		}
	})
	t.Run("given ffi MetricDot, then proto METRIC_DOT", func(t *testing.T) {
		got := ffiMetricToProto(ffi.MetricDot)
		if got != nekov1.Metric_METRIC_DOT {
			t.Errorf("got %v, want %v", got, nekov1.Metric_METRIC_DOT)
		}
	})
	t.Run("given unknown ffi code, then proto METRIC_UNSPECIFIED", func(t *testing.T) {
		got := ffiMetricToProto(99)
		if got != nekov1.Metric_METRIC_UNSPECIFIED {
			t.Errorf("got %v, want %v", got, nekov1.Metric_METRIC_UNSPECIFIED)
		}
	})
}
