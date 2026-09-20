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
)

func NewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name> --dim <N> [--metric <metric>] [--model <model>]",
		Short: "Create a new collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			metricCode, err := ffi.ParseMetric(createMetric)
			if err != nil {
				return err
			}
			if err := ffi.Create(name, createDim, metricCode, createModel); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "collection '%s' created (dim=%d, metric=%s)\n", name, createDim, createMetric)
			return nil
		},
	}

	cmd.Flags().Uint32Var(&createDim, "dim", 0, "vector dimension")
	cmd.Flags().StringVar(&createMetric, "metric", "cosine", "distance metric: l2, cosine, dot")
	cmd.Flags().StringVar(&createModel, "model", "", "model name (optional, future use)")
	cmd.MarkFlagRequired("dim")

	return cmd
}
