package cli

import (
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

func NewDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop <name>",
		Short: "Remove a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ffi.Drop(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "collection '%s' dropped\n", args[0])
			return nil
		},
	}
}
