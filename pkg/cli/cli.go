package cli

import (
	"fmt"
	"os"
	"unsafe"

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

	rootCmd.AddCommand(
		newVersionCmd(),
		newCreateCmd(),
		newListCmd(),
		newDropCmd(),
		newInsertCmd(),
		newGetCmd(),
	)
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

func newInsertCmd() *cobra.Command {
	var (
		insertId   string
		insertFile string
	)

	cmd := &cobra.Command{
		Use:   "insert <collection> --id <ID> --file <vec.f32>",
		Short: "Insert a vector into a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureEngine(); err != nil {
				return err
			}
			name := args[0]
			data, err := os.ReadFile(insertFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %w", insertFile, err)
			}
			if len(data)%4 != 0 {
				return fmt.Errorf("file '%s' has invalid size: must be a multiple of 4 bytes (raw f32)", insertFile)
			}

			floats := unsafe.Slice((*float32)(unsafe.Pointer(&data[0])), len(data)/4)
			if err := ffi.Insert(name, insertId, floats, ""); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "vector '%s' inserted into '%s' (dim=%d)\n", insertId, name, len(floats))
			return nil
		},
	}

	cmd.Flags().StringVar(&insertId, "id", "", "vector ID (required)")
	cmd.Flags().StringVar(&insertFile, "file", "", "path to raw f32 vector file (required)")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("file")
	return cmd
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <collection> <id>",
		Short: "Retrieve a vector by ID",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureEngine(); err != nil {
				return err
			}

			name := args[0]
			id := args[1]

			stats, err := ffi.Stats(name)
			if err != nil {
				return err
			}

			vector, err := ffi.Get(name, id, stats.Dim)
			if err != nil {
				return err
			}

			for i, val := range vector {
				if i > 0 {
					fmt.Fprint(cmd.OutOrStdout(), ",")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%g", val)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
}

func ensureEngine() error {
	return ffi.Init(ffi.DefaultDataDirectory())
}
