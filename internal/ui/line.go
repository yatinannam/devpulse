package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Span is a run of text drawn in one style. A nil Style means the default
// foreground.
type Span struct {
	Text  string
	Style Style
}

// Line is a sequence of styled spans. Its width is the rune count of its
// plain text; box-drawing and glyph characters count as one column.
type Line []Span

// Plain returns a line of unstyled text. Control characters are replaced, as
// described at sanitize.
func Plain(text string) Line { return Line{{Text: sanitize(text)}} }

// Styled returns a line drawn in one style. Control characters are replaced,
// as described at sanitize.
func Styled(text string, style Style) Line { return Line{{Text: sanitize(text), Style: style}} }

// sanitize replaces C0 control characters (including tab and newline), DEL
// and C1 control characters with U+FFFD, and so does invalid UTF-8. Paths,
// methods and process names come from session files and the OS: a raw ESC
// in one could clear the screen or retitle the terminal, and a newline would
// split a table row. Every rune maps to one rune, so widths are unchanged.
func sanitize(text string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return utf8.RuneError
		}
		return r
	}, text)
}

// Join concatenates lines into one.
func Join(parts ...Line) Line {
	var out Line
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Plain returns the line's text without styles.
func (l Line) Plain() string {
	var b strings.Builder
	for _, s := range l {
		b.WriteString(s.Text)
	}
	return b.String()
}

// String returns the line with its styles applied.
func (l Line) String() string {
	var b strings.Builder
	for _, s := range l {
		if s.Style != nil {
			b.WriteString(s.Style(s.Text))
		} else {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

// Width returns the display width of the line.
func (l Line) Width() int {
	n := 0
	for _, s := range l {
		n += utf8.RuneCountInString(s.Text)
	}
	return n
}

// Slice returns the runes in [start, end), keeping their styles.
func (l Line) Slice(start, end int) Line {
	var out Line
	pos := 0
	for _, s := range l {
		runes := []rune(s.Text)
		from, to := pos, pos+len(runes)
		pos = to
		lo, hi := max(from, start), min(to, end)
		if lo >= hi {
			continue
		}
		out = append(out, Span{Text: string(runes[lo-from : hi-from]), Style: s.Style})
	}
	return out
}

// Truncate shortens the line to at most n columns, ending it with "…" in
// the style of the last character kept.
func (l Line) Truncate(n int) Line {
	if l.Width() <= n {
		return l
	}
	if n <= 0 {
		return nil
	}
	out := l.Slice(0, n-1).TrimRight()
	if len(out) == 0 {
		return Line{{Text: "…", Style: l[0].Style}}
	}
	out[len(out)-1].Text += "…"
	return out
}

// TrimRight drops trailing spaces.
func (l Line) TrimRight() Line {
	out := append(Line(nil), l...)
	for len(out) > 0 {
		last := &out[len(out)-1]
		last.Text = strings.TrimRight(last.Text, " ")
		if last.Text != "" {
			break
		}
		out = out[:len(out)-1]
	}
	return out
}

// Wrap breaks the line into lines of at most width columns, at spaces where
// possible and inside a word when one does not fit. Newlines force a break.
func (l Line) Wrap(width int) []Line {
	var out []Line
	for _, r := range wrapRanges([]rune(l.Plain()), width) {
		out = append(out, l.Slice(r[0], r[1]))
	}
	return out
}

// Writeln writes the line and a newline, without trailing spaces.
func Writeln(w io.Writer, l Line) {
	fmt.Fprintln(w, l.TrimRight().String())
}

// Width returns the display width of s, ignoring ANSI escape sequences.
func Width(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		n++
	}
	return n
}

// Truncate shortens plain text to at most n columns, ending it with "…".
func Truncate(s string, n int) string {
	return Plain(s).Truncate(n).Plain()
}

// TruncatePath shortens a file path to at most n columns by dropping its
// start, so the file name stays visible: "…my-app\.devpulse\config.json".
func TruncatePath(path string, n int) string {
	runes := []rune(path)
	if len(runes) <= n {
		return path
	}
	if n <= 0 {
		return ""
	}
	return "…" + string(runes[len(runes)-(n-1):])
}

// Wrap breaks plain text into lines of at most width columns. Newlines in
// text still force a break; other control characters are replaced.
func Wrap(text string, width int) []string {
	var out []string
	for _, paragraph := range strings.Split(text, "\n") {
		for _, l := range Plain(paragraph).Wrap(width) {
			out = append(out, l.Plain())
		}
	}
	return out
}

// wrapRanges returns the [start, end) rune ranges of each wrapped line.
// Every paragraph, even an empty one, yields at least one range.
func wrapRanges(r []rune, width int) [][2]int {
	width = max(1, width)
	var out [][2]int
	start := 0
	for i := 0; i <= len(r); i++ {
		if i < len(r) && r[i] != '\n' {
			continue
		}
		lines := wrapParagraph(r, start, i, width)
		if len(lines) == 0 {
			lines = [][2]int{{start, start}}
		}
		out = append(out, lines...)
		start = i + 1
	}
	return out
}

func wrapParagraph(r []rune, i, n, width int) [][2]int {
	var out [][2]int
	for {
		for i < n && r[i] == ' ' {
			i++
		}
		if i >= n {
			return out
		}
		if n-i <= width {
			end := n
			for end > i && r[end-1] == ' ' {
				end--
			}
			return append(out, [2]int{i, end})
		}
		limit := i + width
		brk := -1
		for j := limit; j > i; j-- {
			if r[j] == ' ' {
				brk = j
				break
			}
		}
		if brk < 0 {
			out = append(out, [2]int{i, limit})
			i = limit
			continue
		}
		end := brk
		for end > i && r[end-1] == ' ' {
			end--
		}
		out = append(out, [2]int{i, end})
		i = brk
	}
}
