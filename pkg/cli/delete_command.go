package cli

import (
	"fmt"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <collection> <id>",
		Short: "Delete a vector by ID",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			id := args[1]
			if err := ffi.Delete(name, id); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "vector '%s' deleted from '%s'\n", id, name)
			return nil
		},
	}
}
