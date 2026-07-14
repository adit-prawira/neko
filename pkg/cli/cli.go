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

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "neko",
		Short:         "neko - a local-first vector database that purrs on your machine",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.AddCommand(newVersionCmd(), newCreateCmd(), newListCmd(), newDropCmd())
	return rootCmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print neko version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "neko v0.1.0")
			return nil
		},
	}
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name> --dim <N> [--metric <metric>] [--model <model>]",
		Short: "Create a new collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureEngine(); err != nil {
				return err
			}
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

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all collections",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureEngine(); err != nil {
				return err
			}
			names, err := ffi.List()
			if err != nil {
				return err
			}
			for _, name := range names {
				stats, err := ffi.Stats(name)
				if err != nil {
					continue
				}
				label := ffi.MetricNames[stats.Metric]
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %4d  %-8s %d vectors\n", name, stats.Dim, label, stats.VectorCount)
			}
			return nil
		},
	}
}

func newDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop <name>",
		Short: "Remove a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureEngine(); err != nil {
				return err
			}
			if err := ffi.Drop(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "collection '%s' dropped\n", args[0])
			return nil
		},
	}
}

func ensureEngine() error {
	return ffi.Init(ffi.DefaultDataDirectory())
}
