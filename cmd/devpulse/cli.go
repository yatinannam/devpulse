package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
)

// parseArgs parses command flags and rejects positional arguments. The flag
// package stops at the first non-flag word and silently ignores it and every
// flag after it, so `devpulse validate ci/session.json` would otherwise check
// the default session instead of the file the user named.
func parseArgs(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q: pass values with flags such as --from, and put all flags before any other text", flags.Arg(0))
	}
	return nil
}

// mustParseArgs is parseArgs for command entry points: usage errors exit 2.
func mustParseArgs(flags *flag.FlagSet, args []string) {
	if err := parseArgs(flags, args); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse %s: %v\n", flags.Name(), err)
		os.Exit(2)
	}
}

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
