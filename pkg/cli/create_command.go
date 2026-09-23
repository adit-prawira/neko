package cli

import (
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

var (
	createDim    uint32
	createMetric string
	createModel  string
	createIndex  string
)

func NewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name> --dim <N> [--metric <metric>] [--model <model>] [--index <type>]",
		Short: "Create a new collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			metricCode, err := ffi.ParseMetric(createMetric)
			if err != nil {
				return err
			}
			indexType, err := parseIndexType(createIndex)
			if err != nil {
				return err
			}
			if err := ffi.Create(name, createDim, metricCode, createModel, indexType); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "collection '%s' created (dim=%d, metric=%s, index=%s)\n", name, createDim, createMetric, createIndex)
			return nil
		},
	}

	cmd.Flags().Uint32Var(&createDim, "dim", 0, "vector dimension")
	cmd.Flags().StringVar(&createMetric, "metric", "cosine", "distance metric: l2, cosine, dot")
	cmd.Flags().StringVar(&createModel, "model", "", "model name (optional, future use)")
	cmd.Flags().StringVar(&createIndex, "index", "brute", "index type: brute or hnsw")
	cmd.MarkFlagRequired("dim")

	return cmd
}

func parseIndexType(value string) (uint8, error) {
	switch value {
	case "brute", "":
		return ffi.IndexTypeBrute, nil
	case "hnsw":
		return ffi.IndexTypeHnsw, nil
	default:
		return 0, fmt.Errorf("invalid --index %q: must be 'brute' or 'hnsw'", value)
	}
}
