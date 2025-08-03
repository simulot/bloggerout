package youtube

import (
	"context"
	"encoding/csv"
	"io"

	"bloggerout/internal/virtualfs"

	"github.com/simulot/TakeoutLocalization/go/localization"
)

// readCSV reads a CSV file and processes each row using the provided yield function.
// when the localization Node is provided, column headers are globalized
func readCSV(
	ctx context.Context,
	vfs virtualfs.FileSystem,
	filePath string,
	n *localization.Node,
	yeld func(cols map[string]int, r []string),
) error {
	file, err := vfs.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	r, err := reader.Read()
	if err != nil {
		return err
	}

	colOfInterest := map[string]int{}
	for i := range r {
		header := r[i]
		if n != nil {
			if key, ok := n.GetColumnKey(header); ok {
				header = key
			}
		}
		colOfInterest[header] = i
	}
read:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			r, err = reader.Read()
			if err != nil {
				break read
			}
			yeld(colOfInterest, r)
		}
	}
	if err != io.EOF {
		return err
	}
	return nil
}
