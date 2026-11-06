package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Title writes the first line of every command: "devpulse <command>", with
// "devpulse" in the accent color and the command in bold. Context is drawn
// dim, right-aligned on the same line when it fits and on the next line
// otherwise. One blank line follows.
func Title(w io.Writer, width int, command string, context Line) {
	left := Styled("devpulse", Accent)
	if command != "" {
		left = Join(left, Plain(" "), Styled(command, Bold))
	}
	ctx := make(Line, len(context))
	for i, s := range context {
		if s.Style == nil {
			s.Style = Muted
		}
		ctx[i] = s
	}
	switch {
	case ctx.Width() == 0:
		Writeln(w, left)
	case left.Width()+2+ctx.Width() <= width:
		Writeln(w, Join(left, Plain(strings.Repeat(" ", width-left.Width()-ctx.Width())), ctx))
	default:
		Writeln(w, left)
		Writeln(w, ctx.Truncate(width))
	}
	fmt.Fprintln(w)
}

// Heading writes a bold section heading with an optional dim note, such as
// a count.
func Heading(w io.Writer, text, note string) {
	l := Styled(text, Bold)
	if note != "" {
		l = Join(l, Styled(" "+note, Muted))
	}
	Writeln(w, l)
}

// KV is one key–value pair.
type KV struct {
	Key   string
	Value Line
}

// KVList writes pairs with their values aligned. Keys are dim; values that
// do not fit are truncated.
func KVList(w io.Writer, width, indent int, pairs []KV) {
	keyWidth := 0
	for _, p := range pairs {
		keyWidth = max(keyWidth, utf8.RuneCountInString(p.Key))
	}
	pad := strings.Repeat(" ", indent)
	for _, p := range pairs {
		space := strings.Repeat(" ", keyWidth-utf8.RuneCountInString(p.Key)+2)
		value := p.Value.Truncate(width - indent - keyWidth - 2)
		Writeln(w, Join(Plain(pad), Styled(p.Key, Muted), Plain(space), value))
	}
}

// Facts writes a bold label followed by short facts three spaces apart,
// wrapping onto lines aligned under the first fact.
func Facts(w io.Writer, width int, label string, facts []Line) {
	indent := utf8.RuneCountInString(label) + 3
	line := Join(Styled(label, Bold), Plain("   "))
	used, empty := indent, true
	for _, f := range facts {
		f = f.Truncate(width - indent)
		if !empty && used+3+f.Width() > width {
			Writeln(w, line)
			line, used, empty = Plain(strings.Repeat(" ", indent)), indent, true
		}
		if !empty {
			line = Join(line, Plain("   "))
			used += 3
		}
		line = Join(line, f)
		used += f.Width()
		empty = false
	}
	Writeln(w, line)
}

// Grid writes short items in as many aligned columns as fit, filling rows
// left to right. Items longer than 24 columns are truncated.
func Grid(w io.Writer, width, indent int, items []Line) {
	if len(items) == 0 {
		return
	}
	const gap = 2
	cellLimit := min(24, width-indent)
	cells := make([]Line, len(items))
	widest := 0
	for i, item := range items {
		cells[i] = item.Truncate(cellLimit)
		widest = max(widest, cells[i].Width())
	}
	cols, widths := 1, []int{widest}
	for c := len(cells); c > 1; c-- {
		ws := make([]int, c)
		for i, cell := range cells {
			ws[i%c] = max(ws[i%c], cell.Width())
		}
		total := indent + gap*(c-1)
		for _, x := range ws {
			total += x
		}
		if total <= width {
			cols, widths = c, ws
			break
		}
	}
	for start := 0; start < len(cells); start += cols {
		line := Plain(strings.Repeat(" ", indent))
		for j := 0; j < cols && start+j < len(cells); j++ {
			cell := cells[start+j]
			if j > 0 {
				line = Join(line, Plain(strings.Repeat(" ", gap)))
			}
			line = Join(line, cell, Plain(strings.Repeat(" ", widths[j]-cell.Width())))
		}
		Writeln(w, line)
	}
}

// Wrapped writes a line wrapped to the width with every line indented.
func Wrapped(w io.Writer, width, indent int, l Line) {
	pad := Plain(strings.Repeat(" ", indent))
	for _, part := range l.Wrap(width - indent) {
		Writeln(w, Join(pad, part))
	}
}

// Marked writes a level glyph followed by text, wrapped with continuation
// lines aligned under the text.
func Marked(w io.Writer, width int, level Level, text Line) {
	for i, part := range text.Wrap(width - 2) {
		prefix := Plain("  ")
		if i == 0 {
			prefix = Join(level.Glyph(), Plain(" "))
		}
		Writeln(w, Join(prefix, part))
	}
}
