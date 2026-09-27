package cli

import (
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

var (
	createDim            uint32
	createMetric         string
	createModel          string
	createIndex          string
	createMaxConnections uint32
	createEfConstruction uint32
)

func NewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name> --dim <N> [--metric <metric>] [--model <model>] [--index <type>] [--max-connections <N>] [--ef-construction <N>]",
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

			if err := validateHnswTuning(cmd, indexType); err != nil {
				return err
			}

			if indexType == ffi.IndexTypeHnsw {
				if err := ffi.CreateWithHnsw(name, createDim, metricCode, createModel, createMaxConnections, createEfConstruction); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "collection '%s' created (dim=%d, metric=%s, index=%s, max_connections=%d, ef_construction=%d)\n", name, createDim, createMetric, createIndex, createMaxConnections, createEfConstruction)
			} else {
				if err := ffi.Create(name, createDim, metricCode, createModel); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "collection '%s' created (dim=%d, metric=%s, index=%s)\n", name, createDim, createMetric, createIndex)
			}
			return nil
		},
	}

	cmd.Flags().Uint32Var(&createDim, "dim", 0, "vector dimension")
	cmd.Flags().StringVar(&createMetric, "metric", "cosine", "distance metric: l2, cosine, dot")
	cmd.Flags().StringVar(&createModel, "model", "", "model name (optional, future use)")
	cmd.Flags().StringVar(&createIndex, "index", "brute", "index type: brute or hnsw")
	cmd.Flags().Uint32Var(&createMaxConnections, "max-connections", 16, "HNSW max connections per noe (hnsw only)")
	cmd.Flags().Uint32Var(&createEfConstruction, "ef-construction", 299, "HNSW construction beam width (hnsw only)")

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

func validateHnswTuning(cmd *cobra.Command, indexType uint8) error {
	if indexType == ffi.IndexTypeBrute {
		if cmd.Flags().Lookup("max-connections").Changed {
			return fmt.Errorf("--max-connections requires --index hnsw")
		}
		if cmd.Flags().Lookup("ef-construction").Changed {
			return fmt.Errorf("--ef-construction requires --index hnsw")
		}
		return nil
	}

	// HNSW: validate the parsed values.
	if createMaxConnections == 0 {
		return fmt.Errorf("--max-connections must be > 0")
	}
	if createEfConstruction == 0 {
		return fmt.Errorf("--ef-construction must be > 0")
	}
	return nil
}
