package cli

import (
	"github.com/adit-prawira/neko/internal/config"
	"github.com/spf13/cobra"
)

var configInitForce bool

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage neko configuration",
	}

	cmd.AddCommand(NewConfigInitCmd())
	return cmd
}

func NewConfigInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a default config.toml under the resolved data directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return config.WriteDefault(resolveConfigPath(), configInitForce)
		},
	}

	cmd.Flags().BoolVar(&configInitForce, "force", false, "Overwrite config.toml if it already exists")
	return cmd
}
