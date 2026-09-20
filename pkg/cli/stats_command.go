package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/spf13/cobra"
)

var statsJSONOut bool

type statsRow struct {
	Name         string `json:"name"`
	Dim          uint32 `json:"dim"`
	Metric       string `json:"metric"`
	VectorCount  uint64 `json:"vector_count"`
	StorageBytes uint64 `json:"storage_bytes"`
	Segments     uint32 `json:"segments"`
}

func NewStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show per-collection vector count, disk usage, segment count",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDirectory := resolveDataDirectory()
			names, err := ffi.List()

			if err != nil {
				return err
			}

			rows := make([]statsRow, 0, len(names))

			for _, name := range names {
				stat, err := ffi.Stats(name)
				if err != nil {
					continue
				}

				rows = append(rows, statsRow{
					Name:         name,
					Dim:          stat.Dim,
					Metric:       ffi.MetricNames[stat.Metric],
					VectorCount:  stat.VectorCount,
					StorageBytes: stat.StorageBytes,
					Segments:     countSegments(resolvedDirectory, name),
				})
			}
			return renderStats(cmd.OutOrStdout(), rows, statsJSONOut)
		},
	}

	cmd.Flags().BoolVar(&statsJSONOut, "json", false, "Emit JSON instead of a table")
	return cmd
}

func countSegments(dataDirectory, name string) uint32 {
	root := filepath.Join(dataDirectory, "collections", name)
	var count uint32
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		switch filepath.Ext(p) {
		case ".vec", ".meta", ".vix":
			count++
		}
		return nil
	})
	return count
}

func renderStats(out io.Writer, rows []statsRow, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(out).Encode(map[string]any{"collections": rows})
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Collection\tDim\tVectors\tDisk\tSegments")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%d\n",
			r.Name, r.Dim, r.VectorCount, formatBytes(r.StorageBytes), r.Segments)
	}

	return w.Flush()
}

func formatBytes(n uint64) string {
	const k = uint64(1024)
	switch {
	case n < k:
		return fmt.Sprintf("%dB", n)
	case n < k*k:
		return fmt.Sprintf("%dKB", n/k)
	case n < k*k*k:
		return fmt.Sprintf("%dMB", n/(k*k))
	default:
		return fmt.Sprintf("%dGB", n/(k*k*k))
	}
}
