package ports

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/yatinannam/devpulse/internal/ui"
)

// Print writes the listener table to stdout, sized to the terminal.
func Print(entries []Entry) {
	Render(os.Stdout, entries, ui.TermWidth())
}

// Render writes a Port, Process, PID table sorted by port, then the
// listener count. It sorts a copy, so the caller's slice keeps its order.
// Every entry is a listener, so no state column is printed.
func Render(w io.Writer, entries []Entry, width int) {
	sorted := append([]Entry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Port < sorted[j].Port })

	ui.Title(w, width, "ports", nil)
	if len(sorted) == 0 {
		fmt.Fprintln(w, "No TCP listeners found.")
		return
	}
	table := ui.NewTable(
		ui.Column{Header: "Port", Align: ui.AlignRight},
		ui.Column{Header: "Process", Flex: true},
		ui.Column{Header: "PID", Align: ui.AlignRight},
	)
	for _, e := range sorted {
		process := ui.Plain(e.Process)
		if e.Process == "" {
			process = ui.Styled("-", ui.Muted)
		}
		table.Row(ui.Plain(strconv.Itoa(e.Port)), process, ui.Plain(e.PID))
	}
	table.Render(w, width)
	fmt.Fprintln(w)
	ui.Writeln(w, ui.Styled(ui.Count(len(sorted), "listener", "listeners"), ui.Muted))
}

// printChange is the stdout form of RenderChange that PrintChange uses.
func printChange(added, removed []Entry) {
	RenderChange(os.Stdout, time.Now(), ui.TermWidth(), added, removed)
}

// RenderChange writes one line per closed port (a red -) and opened port
// (a green +), such as "10:01:02  + :5173  node (pid 9912) opened", in port
// order. On the same port the closed process comes first, so a restart that
// changes a port's PID reads as the old process closing and the new one
// opening.
func RenderChange(w io.Writer, now time.Time, width int, added, removed []Entry) {
	type change struct {
		entry  Entry
		closed bool
	}
	changes := make([]change, 0, len(added)+len(removed))
	for _, e := range removed {
		changes = append(changes, change{e, true})
	}
	for _, e := range added {
		changes = append(changes, change{e, false})
	}
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].entry.Port != changes[j].entry.Port {
			return changes[i].entry.Port < changes[j].entry.Port
		}
		return changes[i].closed && !changes[j].closed
	})

	stamp := now.Format("15:04:05")
	for _, c := range changes {
		if c.closed {
			writeChange(w, width, stamp, ui.Styled("-", ui.Error), c.entry, "closed")
		} else {
			writeChange(w, width, stamp, ui.Styled("+", ui.OK), c.entry, "opened")
		}
	}
}

func writeChange(w io.Writer, width int, stamp string, sign ui.Line, e Entry, verb string) {
	head := ui.Join(ui.Styled(stamp, ui.Muted), ui.Plain("  "), sign, ui.Plain(fmt.Sprintf(" :%-5d ", e.Port)))
	tail := ui.Plain(" " + verb)
	owner := ""
	switch {
	case e.Process != "" && e.PID != "":
		owner = fmt.Sprintf("%s (pid %s)", e.Process, e.PID)
	case e.Process != "":
		owner = e.Process
	case e.PID != "":
		owner = "pid " + e.PID
	}
	if owner == "" {
		ui.Writeln(w, ui.Join(head.TrimRight(), tail))
		return
	}
	ui.Writeln(w, ui.Join(head, ui.Plain(owner).Truncate(width-head.Width()-tail.Width()), tail))
}
