package grpcserver

import (
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	nekov1 "github.com/adit-prawira/neko/internal/gen/neko/v1"
)

func protoMetricToFFI(metric nekov1.Metric) (uint8, error) {
	switch metric {
	case nekov1.Metric_METRIC_COSINE:
		return ffi.MetricCosine, nil
	case nekov1.Metric_METRIC_DOT:
		return ffi.MetricDot, nil
	case nekov1.Metric_METRIC_L2:
		return ffi.MetricL2, nil
	default:
		return 0, fmt.Errorf("invalid metric: %s", metric)
	}
}

func ffiMetricToProto(metric uint8) nekov1.Metric {
	switch metric {
	case ffi.MetricCosine:
		return nekov1.Metric_METRIC_COSINE
	case ffi.MetricDot:
		return nekov1.Metric_METRIC_DOT
	case ffi.MetricL2:
		return nekov1.Metric_METRIC_L2
	default:
		return nekov1.Metric_METRIC_UNSPECIFIED
	}
}
