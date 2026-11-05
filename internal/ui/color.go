// Package ui renders DevPulse's terminal output: color tokens that respect
// the user's terminal theme, terminal width detection, and the layout
// primitives every command shares (title lines, tables, grids, wrapped text
// and the pulse sparkline).
//
// Layout always works on plain text, so widths are measured before any
// escape codes are added, and every style returns its input unchanged when
// color is off.
package ui

import (
	"os"
	"strings"
	"sync"
)

// Style decorates text for the terminal.
type Style func(string) string

var (
	colorOnce sync.Once
	colorOn   bool
)

// ColorEnabled reports whether styles emit ANSI escapes. The policy is
// resolved once, on first use, from the environment and stdout.
func ColorEnabled() bool {
	colorOnce.Do(func() {
		colorOn = decideColor(os.LookupEnv, IsTerminal(), enableVirtualTerminal)
	})
	return colorOn
}

// SetColor overrides the detected color policy. Tests use it to make output
// deterministic.
func SetColor(on bool) {
	colorOnce.Do(func() {})
	colorOn = on
}

// decideColor applies the color policy:
//
//   - DEVPULSE_COLOR=always forces color on and DEVPULSE_COLOR=never forces
//     it off; any other value falls through to detection.
//   - NO_COLOR (set to any value) or TERM=dumb turns color off.
//   - Color is off when stdout is not a terminal.
//   - Otherwise color is on if the terminal accepts escape sequences, which
//     on Windows means virtual-terminal processing could be enabled.
func decideColor(lookup func(string) (string, bool), tty bool, enableVT func() bool) bool {
	if v, ok := lookup("DEVPULSE_COLOR"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "always":
			enableVT()
			return true
		case "never":
			return false
		}
	}
	if _, ok := lookup("NO_COLOR"); ok {
		return false
	}
	if term, _ := lookup("TERM"); term == "dumb" {
		return false
	}
	if !tty {
		return false
	}
	return enableVT()
}

func paint(code, s string) string {
	if s == "" || !ColorEnabled() {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// The color tokens use the 16 base ANSI colors only, so the user's terminal
// theme decides the exact shade.

// OK marks healthy state: 2xx responses and the ● glyph.
func OK(s string) string { return paint("32", s) }

// Warn marks things worth a look: 4xx responses, slow requests, ▲.
func Warn(s string) string { return paint("33", s) }

// Error marks what is wrong: 5xx responses, very slow requests, ✖.
func Error(s string) string { return paint("31", s) }

// Muted recedes: table headers, labels, context and idle listeners.
func Muted(s string) string { return paint("90", s) }

// Bold is for titles and key numbers only.
func Bold(s string) string { return paint("1", s) }

// Accent is reserved for the word "devpulse" in title lines and for the
// pulse sparkline. Nothing else uses it.
func Accent(s string) string { return paint("35", s) }

// Normal leaves text in the terminal's default foreground.
func Normal(s string) string { return s }
