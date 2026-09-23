package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

var (
	benchVectors uint32
	benchDim     uint32
	benchK       uint32
	benchQueries uint32
	benchMetric  string
	benchJSON    bool
)

type BenchResult struct {
	Metric          string  `json:"metric"`
	Dim             uint32  `json:"dim"`
	Vectors         uint32  `json:"vectors"`
	K               uint32  `json:"k"`
	Queries         uint32  `json:"queries"`
	InsertSeconds   float64 `json:"insert_seconds"`
	InsertPerSecond float64 `json:"insert_per_second"`
	SearchQPS       float64 `json:"search_qps"`
	SearchP50Millis float64 `json:"search_p50_millis"`
	SearchP95Millis float64 `json:"search_p95_millis"`
	SearchP99Millis float64 `json:"search_p99_millis"`
}

func NewBenchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark insert and search throughout on a temporary collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			metricCode, err := ffi.ParseMetric(benchMetric)
			if err != nil {
				return err
			}

			collectionName := fmt.Sprintf("bench_%d", time.Now().UnixNano())
			if err := ffi.Create(collectionName, benchDim, metricCode, "", ffi.IndexTypeBrute); err != nil {
				return err
			}

			defer func() {
				_ = ffi.Drop(collectionName)
			}()

			vectors := make([][]float32, benchVectors)
			for i := range vectors {
				vectors[i] = randomVector(benchDim)
			}

			inputVectors := make([]ffi.InputVector, benchVectors)
			for i, vec := range vectors {
				inputVectors[i] = ffi.InputVector{
					ID:     fmt.Sprintf("v%d", i),
					Vector: vec,
				}
			}
			insertStart := time.Now()
			if err := ffi.InsertMany(collectionName, inputVectors); err != nil {
				return err
			}
			insertElapsed := time.Since(insertStart)
			queries := make([][]float32, benchQueries)
			for i := range queries {
				queries[i] = randomVector(benchDim)
			}

			latencies := make([]time.Duration, benchQueries)
			for i, q := range queries {
				start := time.Now()
				if _, err := ffi.Search(collectionName, q, benchK); err != nil {
					return err
				}
				latencies[i] = time.Since(start)
			}

			slices.Sort(latencies)
			p50 := latencies[len(latencies)*50/100]
			p95 := latencies[len(latencies)*95/100]
			p99 := latencies[len(latencies)*99/100]
			var totalSearch time.Duration

			for _, d := range latencies {
				totalSearch += d
			}

			result := BenchResult{
				Metric:          benchMetric,
				Dim:             benchDim,
				Vectors:         benchVectors,
				K:               benchK,
				Queries:         benchQueries,
				InsertSeconds:   insertElapsed.Seconds(),
				InsertPerSecond: float64(benchVectors) / insertElapsed.Seconds(),
				SearchQPS:       float64(benchQueries) / totalSearch.Seconds(),
				SearchP50Millis: float64(p50.Microseconds()) / 1000.0,
				SearchP95Millis: float64(p95.Microseconds()) / 1000.0,
				SearchP99Millis: float64(p99.Microseconds()) / 1000.0,
			}

			return renderBench(cmd.OutOrStdout(), result, benchJSON)
		},
	}
	cmd.Flags().Uint32Var(&benchVectors, "vectors", 100000, "number of vectors to insert")
	cmd.Flags().Uint32Var(&benchDim, "dim", 384, "vector dimension")
	cmd.Flags().Uint32Var(&benchK, "k", 10, "top-K for search")
	cmd.Flags().Uint32Var(&benchQueries, "queries", 1000, "number of search queries for latency stats")
	cmd.Flags().StringVar(&benchMetric, "metric", "cosine", "distance metric: l2, cosine, or dot")
	cmd.Flags().BoolVar(&benchJSON, "json", false, "Emit JSON instead of formatted text")
	return cmd
}

func randomVector(dim uint32) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = rand.Float32()*2 - 1
	}
	return v
}

func renderBench(out io.Writer, result BenchResult, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(out).Encode(result)
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Metric:\t%s\n", result.Metric)
	fmt.Fprintf(w, "Vectors:\t%d (dim=%d)\n", result.Vectors, result.Dim)
	fmt.Fprintf(w, "Insert:\t%.2fs (%.0f vec/sec)\n", result.InsertSeconds, result.InsertPerSecond)
	fmt.Fprintf(w, "Search:\t%.0f QPS (k=%d, queries=%d)\n", result.SearchQPS, result.K, result.Queries)
	fmt.Fprintf(w, "  p50:\t%.2f ms\n", result.SearchP50Millis)
	fmt.Fprintf(w, "  p95:\t%.2f ms\n", result.SearchP95Millis)
	fmt.Fprintf(w, "  p99:\t%.2f ms\n", result.SearchP99Millis)
	return w.Flush()
}
