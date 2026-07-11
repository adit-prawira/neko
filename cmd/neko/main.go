package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCommand = &cobra.Command{
	Use:   "neko",
	Short: "neko - 🐱 a local-first vector database that purrs on your machine",
}

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print neko version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("neko v0.1.0")
	},
}

func init() {
	rootCommand.AddCommand(versionCommand)
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
