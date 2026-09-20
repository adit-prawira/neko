package cli

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

var (
	searchFile string
	searchK    uint32
)

func NewSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <collection> --file <query.f32> --k <N>",
		Short: "Search top-K nearest neighbors",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			data, err := os.ReadFile(searchFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %w", searchFile, err)
			}
			if len(data)%4 != 0 {
				return fmt.Errorf("file '%s' has invalid size: must be a multiple of 4 bytes (raw f32)", searchFile)
			}
			floats := unsafe.Slice((*float32)(unsafe.Pointer(&data[0])), len(data)/4)
			results, err := ffi.Search(name, floats, searchK)
			if err != nil {
				return err
			}

			for _, result := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.4f\n", result.ID, result.Score)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&searchFile, "file", "f", "", "path to raw f32 query vector file (required)")
	cmd.Flags().Uint32VarP(&searchK, "k", "k", 10, "number of results")
	cmd.MarkFlagRequired("file")
	return cmd
}
