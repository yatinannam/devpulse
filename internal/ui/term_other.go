//go:build !windows && !linux && !darwin

package ui

// enableVirtualTerminal assumes the terminal interprets ANSI escapes, as
// every Unix terminal does.
func enableVirtualTerminal() bool { return true }

// terminalWidth is not detected on this platform, so callers fall back to
// COLUMNS or the default width.
func terminalWidth() (int, bool) { return 0, false }
