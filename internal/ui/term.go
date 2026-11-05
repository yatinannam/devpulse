package ui

import (
	"os"
	"strconv"
	"strings"
)

const (
	minWidth      = 60
	maxWidth      = 160
	fallbackWidth = 100
)

// IsTerminal reports whether stdout is a terminal rather than a pipe or a
// file.
func IsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// TermWidth returns the width output must fit in: COLUMNS when it is set,
// otherwise the detected terminal width, otherwise 100. The result is
// clamped to 60..160.
func TermWidth() int {
	return resolveWidth(os.Getenv("COLUMNS"), terminalWidth)
}

func resolveWidth(columns string, detect func() (int, bool)) int {
	width := fallbackWidth
	if n, err := strconv.Atoi(strings.TrimSpace(columns)); err == nil && n > 0 {
		width = n
	} else if n, ok := detect(); ok && n > 0 {
		width = n
	}
	return max(minWidth, min(maxWidth, width))
}
