package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ensureDistinctOutput fails when outputPath names the same file as
// sourcePath. It compares file identity rather than path strings, so case
// variants on Windows and macOS, hard links and symlinks are all caught.
func ensureDistinctOutput(sourcePath, outputPath string) error {
	source, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("inspect source session: %w", err)
	}
	output, err := os.Stat(outputPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output path: %w", err)
	}
	if os.SameFile(source, output) {
		return fmt.Errorf("output path must differ from the source session")
	}
	return nil
}
