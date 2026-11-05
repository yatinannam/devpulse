package ui

import (
	"io"
	"strings"
	"unicode/utf8"
)

// Align is a column's horizontal alignment.
type Align int

const (
	// AlignLeft suits text.
	AlignLeft Align = iota
	// AlignRight suits numbers.
	AlignRight
)

// Column describes one table column.
type Column struct {
	// Header is drawn dim above the rows. A table whose headers are all
	// empty has no header row.
	Header string
	Align  Align
	// Flex columns give up width first when the table is too wide; their
	// cells are truncated with "…", or wrapped when Wrap is set. A flex
	// column never grows past its widest cell.
	Flex bool
	Wrap bool
	// Min is the narrowest the column shrinks to before other columns give
	// up width. Zero means the header's width.
	Min int
}

// Table lays out rows in aligned columns that fit the terminal width.
type Table struct {
	Columns []Column
	Indent  int
	Gap     int
	rows    [][]Line
}

// NewTable returns a table with a two-space gap between columns.
func NewTable(columns ...Column) *Table {
	return &Table{Columns: columns, Gap: 2}
}

// Row appends a row. Missing trailing cells are empty.
func (t *Table) Row(cells ...Line) { t.rows = append(t.rows, cells) }

// Len returns the number of rows.
func (t *Table) Len() int { return len(t.rows) }

func (t *Table) hasHeader() bool {
	for _, c := range t.Columns {
		if c.Header != "" {
			return true
		}
	}
	return false
}

// widths returns each column's width. A column with no content and no
// header has width zero and is skipped, gap included.
func (t *Table) widths(limit int) []int {
	w := make([]int, len(t.Columns))
	for i, c := range t.Columns {
		w[i] = utf8.RuneCountInString(c.Header)
	}
	for _, row := range t.rows {
		for i := 0; i < len(w) && i < len(row); i++ {
			w[i] = max(w[i], row[i].Width())
		}
	}
	excess := t.Indent - limit
	visible := 0
	for _, x := range w {
		if x > 0 {
			excess += x
			visible++
		}
	}
	if visible > 1 {
		excess += t.Gap * (visible - 1)
	}
	// Shrink one column at a time, always the widest eligible one: flex
	// columns first, then other text columns, then anything.
	for pass := 0; pass < 3 && excess > 0; pass++ {
		for excess > 0 {
			best := -1
			for i, c := range t.Columns {
				floor := 1
				if pass < 2 {
					floor = c.Min
					if floor == 0 {
						floor = utf8.RuneCountInString(c.Header)
					}
					floor = max(1, floor)
				}
				eligible := pass == 2 || (pass == 0 && c.Flex) || (pass == 1 && c.Align == AlignLeft)
				if eligible && w[i] > floor && (best < 0 || w[i] > w[best]) {
					best = i
				}
			}
			if best < 0 {
				break
			}
			w[best]--
			excess--
		}
	}
	return w
}

// Render writes the table so that no line is wider than width.
func (t *Table) Render(w io.Writer, width int) {
	widths := t.widths(width)
	if t.hasHeader() {
		header := make([]Line, len(t.Columns))
		for i, c := range t.Columns {
			header[i] = Styled(c.Header, Muted)
		}
		t.writeRow(w, widths, header)
	}
	for _, row := range t.rows {
		t.writeRow(w, widths, row)
	}
}

func (t *Table) writeRow(w io.Writer, widths []int, row []Line) {
	cells := make([][]Line, len(t.Columns))
	height := 1
	for i, c := range t.Columns {
		var cell Line
		if i < len(row) {
			cell = row[i]
		}
		if c.Wrap && widths[i] > 0 {
			cells[i] = cell.Wrap(widths[i])
		} else {
			cells[i] = []Line{cell.Truncate(widths[i])}
		}
		height = max(height, len(cells[i]))
	}
	gap := Plain(strings.Repeat(" ", t.Gap))
	for k := 0; k < height; k++ {
		line := Plain(strings.Repeat(" ", t.Indent))
		first := true
		for i, c := range t.Columns {
			if widths[i] == 0 {
				continue
			}
			if !first {
				line = Join(line, gap)
			}
			first = false
			var cell Line
			if k < len(cells[i]) {
				cell = cells[i][k]
			}
			pad := Plain(strings.Repeat(" ", widths[i]-cell.Width()))
			if c.Align == AlignRight {
				line = Join(line, pad, cell)
			} else {
				line = Join(line, cell, pad)
			}
		}
		Writeln(w, line)
	}
}
