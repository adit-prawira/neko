package cli

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

var (
	insertId   string
	insertFile string
)

func NewInsertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insert <collection> --id <ID> --file <vec.f32>",
		Short: "Insert a vector into a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			data, err := os.ReadFile(insertFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %w", insertFile, err)
			}
			if len(data)%4 != 0 {
				return fmt.Errorf("file '%s' has invalid size: must be a multiple of 4 bytes (raw f32)", insertFile)
			}

			floats := unsafe.Slice((*float32)(unsafe.Pointer(&data[0])), len(data)/4)
			if err := ffi.Insert(name, insertId, floats, ""); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "vector '%s' inserted into '%s' (dim=%d)\n", insertId, name, len(floats))
			return nil
		},
	}

	cmd.Flags().StringVar(&insertId, "id", "", "vector ID (required)")
	cmd.Flags().StringVar(&insertFile, "file", "", "path to raw f32 vector file (required)")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("file")
	return cmd
}
