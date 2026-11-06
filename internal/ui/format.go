package ui

import (
	"fmt"
	"strconv"
	"time"
)

// Level is the health a glyph reports.
type Level int

const (
	// LevelIdle is an idle or non-HTTP listener: ○, muted.
	LevelIdle Level = iota
	// LevelOK is healthy: ●, green.
	LevelOK
	// LevelWarn deserves a look: ▲, yellow.
	LevelWarn
	// LevelError is wrong: ✖, red.
	LevelError
)

// Symbol returns the level's glyph without color. Glyph shapes differ, so
// they still read when color is off; callers always pair them with a word.
func (l Level) Symbol() string {
	switch l {
	case LevelOK:
		return "●"
	case LevelWarn:
		return "▲"
	case LevelError:
		return "✖"
	default:
		return "○"
	}
}

// Style returns the color token for the level.
func (l Level) Style() Style {
	switch l {
	case LevelOK:
		return OK
	case LevelWarn:
		return Warn
	case LevelError:
		return Error
	default:
		return Muted
	}
}

// Glyph returns the level's colored glyph.
func (l Level) Glyph() Line { return Styled(l.Symbol(), l.Style()) }

// Slow and VerySlow are the latency thresholds drawn in yellow and red.
const (
	Slow     = 500 * time.Millisecond
	VerySlow = time.Second
)

// Duration formats a latency compactly: "52ms", "1.06s", "1m5s".
func Duration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0ms"
	case d < time.Millisecond:
		return "<1ms"
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	case d < time.Minute:
		return d.Round(10 * time.Millisecond).String()
	default:
		return d.Round(time.Second).String()
	}
}

// LatencyStyle colors latency at or above 500ms yellow and at or above 1s
// red.
func LatencyStyle(d time.Duration) Style {
	switch {
	case d >= VerySlow:
		return Error
	case d >= Slow:
		return Warn
	default:
		return Normal
	}
}

// Latency returns a formatted, colored latency.
func Latency(d time.Duration) Line { return Styled(Duration(d), LatencyStyle(d)) }

// StatusStyle colors HTTP statuses: 2xx green, 3xx default, 4xx yellow,
// 5xx red.
func StatusStyle(code int) Style {
	switch {
	case code >= 500:
		return Error
	case code >= 400:
		return Warn
	case code >= 200 && code < 300:
		return OK
	default:
		return Normal
	}
}

// Status returns a colored HTTP status code.
func Status(code int) Line { return Styled(strconv.Itoa(code), StatusStyle(code)) }

// Count formats n with the singular or plural noun: "1 error", "2 errors".
func Count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
