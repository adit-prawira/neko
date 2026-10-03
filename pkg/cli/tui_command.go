package cli

import (
	"github.com/adit-prawira/neko/internal/tui"
	"github.com/spf13/cobra"
)

func NewTuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Opening an interactive terminal dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run()
		},
	}
}
