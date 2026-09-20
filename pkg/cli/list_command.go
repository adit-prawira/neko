package cli

import (
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all collections",
		RunE: func(cmd *cobra.Command, args []string) error {
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
