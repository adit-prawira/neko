package cli

import (
	"os"
	"path/filepath"

	"github.com/adit-prawira/neko/internal/config"
	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/server"
	"github.com/spf13/cobra"
)

var servePort int

func NewServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the neko REST server",
		Long:  "Start the neko REST server on the configured port with graceful shutdown.",
		RunE: func(cmd *cobra.Command, args []string) error {
			portFlag := cmd.Flags().Lookup("port")
			isPortChanged := portFlag != nil && portFlag.Changed

			resolvedDirectory := resolveDataDirectory()
			isDataDirectoryChanged := dataDirectory != ""
			configPath := filepath.Join(resolvedDirectory, "config.toml")
			loadedConfig, err := config.Load(configPath)
			if err != nil {
				return err
			}
			resolved, err := config.Resolve(loadedConfig, config.ResolveDTO{
				Port: config.Property[int]{
					Value:     servePort,
					IsChanged: isPortChanged,
				},
				DataDirectory: config.Property[string]{
					Value:     resolvedDirectory,
					IsChanged: isDataDirectoryChanged,
				},
				EnvDataDirectory: os.Getenv("NEKO_HOME"),
			})

			if err != nil {
				return err
			}

			if resolved.DataDirectory == "" {
				resolved.DataDirectory = ffi.DefaultDataDirectory()
			}

			return server.Start(server.Config{
				Port:          resolved.Port,
				DataDirectory: resolved.DataDirectory,
			})
		},
	}

	cmd.Flags().IntVar(&servePort, "port", 3434, "HTTP Port")
	return cmd
}
