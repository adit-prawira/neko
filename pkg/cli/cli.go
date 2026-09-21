package cli

import (
	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "neko",
		Short:         "neko - a local-first vector database that purrs on your machine",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Use == "version" {
				return nil
			}

			if cmd.HasParent() && cmd.Parent().Use == "config" {
				return nil
			}

			return ensureEngine()
		},
	}

	rootCmd.PersistentFlags().StringVar(&dataDirectory, "data-dir", "", "Data directory (overrides NEKO_HOME)")
	rootCmd.AddCommand(
		NewVersionCmd(),
		NewCreateCmd(),
		NewListCmd(),
		NewDropCmd(),
		NewInsertCmd(),
		NewGetCmd(),
		NewSearchCmd(),
		NewDeleteCmd(),
		NewUpsertCmd(),
		NewServeCmd(),
		NewStatsCmd(),
		NewBenchCmd(),
		NewConfigCmd(),
	)
	return rootCmd
}

func ensureEngine() error {
	return ffi.Init(resolveDataDirectory())
}
