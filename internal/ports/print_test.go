package ports

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/ui"
)

func TestRenderSortsACopyByPort(t *testing.T) {
	ui.SetColor(false)
	entries := []Entry{
		{Port: 5432, Process: "postgres", PID: "2", State: "LISTENING"},
		{Port: 22, Process: "sshd", PID: "5884", State: "LISTENING"},
		{Port: 100, Process: "SamsungFindWindowsService", PID: "8548", State: "LISTENING"},
		{Port: 3000, PID: "46552", State: "LISTENING"},
	}
	var b strings.Builder
	Render(&b, entries, 80)

	row := func(port, process, pid string) string {
		return strings.TrimRight(fmt.Sprintf("%4s  %-25s  %5s", port, process, pid), " ") + "\n"
	}
	want := "devpulse ports\n\n" +
		row("Port", "Process", "PID") +
		row("22", "sshd", "5884") +
		row("100", "SamsungFindWindowsService", "8548") +
		row("3000", "-", "46552") +
		row("5432", "postgres", "2") +
		"\n4 listeners\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if entries[0].Port != 5432 || entries[1].Port != 22 {
		t.Fatalf("Render reordered the caller's slice: %+v", entries)
	}
}

func TestRenderTruncatesLongProcessNamesToTheWidth(t *testing.T) {
	ui.SetColor(false)
	entries := []Entry{{Port: 100, Process: "SamsungFindWindowsService" + strings.Repeat("X", 80), PID: "8548"}}
	var b strings.Builder
	Render(&b, entries, 60)
	out := b.String()
	if !strings.Contains(out, "…") {
		t.Fatalf("expected a truncated process name:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if ui.Width(line) > 60 {
			t.Errorf("line exceeds 60 columns: %q", line)
		}
	}
	if !strings.Contains(out, " 8548\n") {
		t.Fatalf("PID column lost:\n%s", out)
	}
}

func TestRenderEmpty(t *testing.T) {
	ui.SetColor(false)
	var b strings.Builder
	Render(&b, nil, 80)
	if want := "devpulse ports\n\nNo TCP listeners found.\n"; b.String() != want {
		t.Fatalf("got %q, want %q", b.String(), want)
	}
}

func TestRenderChange(t *testing.T) {
	ui.SetColor(false)
	now := time.Date(2026, 11, 5, 10, 1, 2, 0, time.Local)
	var b strings.Builder
	RenderChange(&b, now, 80,
		[]Entry{{Port: 5173, Process: "node", PID: "9912"}},
		[]Entry{{Port: 8080, PID: "77"}, {Port: 49664, Process: "lsass", PID: "1520"}})
	want := "" +
		"10:01:02  - :8080  pid 77 closed\n" +
		"10:01:02  - :49664 lsass (pid 1520) closed\n" +
		"10:01:02  + :5173  node (pid 9912) opened\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderChangeShowsARestartAsClosedThenOpened(t *testing.T) {
	ui.SetColor(false)
	now := time.Date(2026, 11, 5, 10, 1, 2, 0, time.Local)
	var b strings.Builder
	RenderChange(&b, now, 80,
		[]Entry{{Port: 5173, Process: "node", PID: "9913"}},
		[]Entry{{Port: 5173, Process: "node", PID: "9912"}})
	want := "" +
		"10:01:02  - :5173  node (pid 9912) closed\n" +
		"10:01:02  + :5173  node (pid 9913) opened\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderChangeColorsTheSign(t *testing.T) {
	ui.SetColor(true)
	defer ui.SetColor(false)
	now := time.Date(2026, 11, 5, 10, 1, 2, 0, time.Local)
	var b strings.Builder
	RenderChange(&b, now, 80, []Entry{{Port: 5173, Process: "node", PID: "9912"}}, []Entry{{Port: 5173, Process: "node", PID: "9912"}})
	out := b.String()
	if !strings.Contains(out, ui.OK("+")) || !strings.Contains(out, ui.Error("-")) {
		t.Fatalf("expected a green + and a red -: %q", out)
	}
}

func TestRenderChangeFitsTheWidth(t *testing.T) {
	ui.SetColor(false)
	now := time.Date(2026, 11, 5, 10, 1, 2, 0, time.Local)
	var b strings.Builder
	RenderChange(&b, now, 60, []Entry{{Port: 5173, Process: strings.Repeat("p", 100), PID: "9912"}}, nil)
	line := strings.TrimRight(b.String(), "\n")
	if ui.Width(line) > 60 || !strings.HasSuffix(line, " opened") {
		t.Fatalf("line should fit 60 columns and keep its verb: %q", line)
	}
}
