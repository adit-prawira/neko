package cli

import (
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <collection> <id>",
		Short: "Retrieve a vector by ID",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
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
