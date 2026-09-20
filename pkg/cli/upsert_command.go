package cli

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

var (
	upsertId   string
	upsertFile string
)

func NewUpsertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upsert <collection> --id <ID> --file <vec.f32>",
		Short: "Insert or update a vector by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			data, err := os.ReadFile(upsertFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %w", upsertFile, err)
			}

			if len(data)%4 != 0 {
				return fmt.Errorf("file '%s' invalid size: must be a multiple of 4 bytes (raw f32)", upsertFile)
			}

			floats := unsafe.Slice((*float32)(unsafe.Pointer(&data[0])), len(data)/4)
			if _, err := ffi.Upsert(name, upsertId, floats, ""); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "vector '%s' upserted into '%s' (dim=%d)\n", upsertId, name, len(floats))
			return nil
		},
	}

	cmd.Flags().StringVar(&upsertId, "id", "", "vector ID (required)")
	cmd.Flags().StringVar(&upsertFile, "file", "", "path to raw f32 vector file (required)")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("file")
	return cmd
}
