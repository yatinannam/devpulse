package ui

import (
	"fmt"
	"strings"
	"testing"
)

func render(f func(b *strings.Builder)) string {
	var b strings.Builder
	f(&b)
	return b.String()
}

func assertFits(t *testing.T, out string, width int) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if Width(line) > width {
			t.Errorf("line is %d wide, limit %d: %q", Width(line), width, line)
		}
	}
}

func TestTableAlignsColumns(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) {
		tb := NewTable(Column{Header: "Port", Align: AlignRight}, Column{Header: "Process", Flex: true}, Column{Header: "PID", Align: AlignRight})
		tb.Row(Plain("22"), Plain("sshd"), Plain("5884"))
		tb.Row(Plain("3000"), Plain("node"), Plain("46552"))
		tb.Render(b, 80)
	})
	want := "" +
		"Port  Process    PID\n" +
		"  22  sshd      5884\n" +
		"3000  node     46552\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTableTruncatesTheFlexColumnToFit(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) {
		tb := NewTable(Column{Header: "Port", Align: AlignRight}, Column{Header: "Process", Flex: true}, Column{Header: "PID", Align: AlignRight})
		tb.Row(Plain("100"), Plain("SamsungFindWindowsService"), Plain("8548"))
		tb.Render(b, 20)
	})
	want := "" +
		"Port  Process    PID\n" +
		" 100  Samsung…  8548\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTableCapsAColumnAtMax(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) {
		tb := NewTable(Column{Flex: true, Max: 8}, Column{Align: AlignRight})
		tb.Row(Plain("/api/orders/:id"), Plain("2"))
		tb.Row(Plain("/"), Plain("10"))
		tb.Render(b, 80)
	})
	want := "" +
		"/api/or…   2\n" +
		"/         10\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTableWrapsAWrapColumn(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) {
		tb := NewTable(Column{Header: "Field"}, Column{Header: "Message", Flex: true, Wrap: true})
		tb.Row(Plain("status"), Plain("HTTP status must be between 100 and 599"))
		tb.Render(b, 30)
	})
	want := "" +
		"Field   Message\n" +
		"status  HTTP status must be\n" +
		"        between 100 and 599\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTableHeaderIsDimAndIndentApplies(t *testing.T) {
	SetColor(true)
	defer SetColor(false)
	got := render(func(b *strings.Builder) {
		tb := NewTable(Column{Header: "Time"}, Column{Header: "Status", Align: AlignRight})
		tb.Indent = 2
		tb.Row(Plain("10:00:01"), Status(500))
		tb.Render(b, 80)
	})
	want := "  " + Muted("Time") + "      " + Muted("Status") + "\n" +
		"  10:00:01     " + Error("500") + "\n"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestTableWithoutHeadersSkipsTheHeaderRowAndEmptyColumns(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) {
		tb := NewTable(Column{}, Column{}, Column{})
		tb.Row(Plain("a"), nil, Plain("b"))
		tb.Render(b, 80)
	})
	if got != "a  b\n" {
		t.Fatalf("got %q", got)
	}
}

func TestTableNeverExceedsTheWidth(t *testing.T) {
	SetColor(false)
	for width := 60; width <= 160; width += 7 {
		out := render(func(b *strings.Builder) {
			tb := NewTable(
				Column{Header: "Time"},
				Column{Header: "Method"},
				Column{Header: "Path", Flex: true},
				Column{Header: "Status", Align: AlignRight},
				Column{Header: "Latency", Align: AlignRight},
			)
			for i := 0; i < 5; i++ {
				tb.Row(Plain("10:00:01"), Plain("DELETE"), Plain("/api/"+strings.Repeat("segment/", 30)+fmt.Sprint(i)), Plain("500"), Plain("1.06s"))
			}
			tb.Render(b, width)
		})
		assertFits(t, out, width)
	}
}

func TestTitleRightAlignsContext(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) { Title(b, 60, "status", Plain("demo.json, 12 requests")) })
	want := "devpulse status" + strings.Repeat(" ", 23) + "demo.json, 12 requests\n\n"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestTitleMovesLongContextToTheNextLine(t *testing.T) {
	SetColor(false)
	context := strings.Repeat("x", 50)
	got := render(func(b *strings.Builder) { Title(b, 60, "status", Plain(context)) })
	if want := "devpulse status\n" + context + "\n\n"; got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestTitleStylesAccentCommandAndDimContext(t *testing.T) {
	SetColor(true)
	defer SetColor(false)
	got := render(func(b *strings.Builder) { Title(b, 60, "doctor", nil) })
	if want := Accent("devpulse") + " " + Bold("doctor") + "\n\n"; got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	got = render(func(b *strings.Builder) { Title(b, 30, "doctor", Plain("demo.json")) })
	if !strings.Contains(got, Muted("demo.json")) {
		t.Fatalf("context should be dim: %q", got)
	}
}

func TestGridFillsRowsInOrder(t *testing.T) {
	SetColor(false)
	items := []Line{Plain(":22 sshd"), Plain(":135 svchost"), Plain(":445 System"), Plain(":3306 mysqld"), Plain(":5040 svchost"), Plain(":7680 svchost")}
	got := render(func(b *strings.Builder) { Grid(b, 50, 2, items) })
	want := "" +
		"  :22 sshd      :135 svchost   :445 System\n" +
		"  :3306 mysqld  :5040 svchost  :7680 svchost\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestGridTruncatesLongItems(t *testing.T) {
	SetColor(false)
	items := []Line{Plain(":100 SamsungFindWindowsServiceWithAnEvenLongerName")}
	out := render(func(b *strings.Builder) { Grid(b, 60, 2, items) })
	if !strings.Contains(out, "…") {
		t.Fatalf("expected a truncated item: %q", out)
	}
	assertFits(t, out, 60)
}

func TestFactsWrapUnderTheFirstFact(t *testing.T) {
	SetColor(false)
	facts := []Line{Plain("12 requests"), Plain("2 errors (16.7%)"), Plain("avg 226ms"), Plain("p50 60ms")}
	got := render(func(b *strings.Builder) { Facts(b, 40, "Traffic", facts) })
	want := "" +
		"Traffic   12 requests   2 errors (16.7%)\n" +
		"          avg 226ms   p50 60ms\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestKVListAlignsValues(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) {
		KVList(b, 60, 0, []KV{{"Requests", Plain("12")}, {"Errors", Plain("3 (25.0%)")}, {"Source", nil}})
	})
	want := "Requests  12\nErrors    3 (25.0%)\nSource\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

func TestMarkedWrapsWithAHangingIndent(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) {
		Marked(b, 30, LevelOK, Plain("No captured traffic yet. Run devpulse traffic."))
	})
	want := "" +
		"● No captured traffic yet. Run\n" +
		"  devpulse traffic.\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestWrappedIndentsEveryLine(t *testing.T) {
	SetColor(false)
	got := render(func(b *strings.Builder) { Wrapped(b, 20, 4, Plain("Check backend logs for the slow endpoint")) })
	want := "" +
		"    Check backend\n" +
		"    logs for the\n" +
		"    slow endpoint\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSparkline(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		width  int
		want   string
	}{
		{"scaled to the maximum", []float64{0, 1, 2, 4, 8}, 10, "▁▁▂▄█"},
		{"resampled by bin maximum", []float64{1, 1, 1, 1, 8, 1, 1, 1}, 4, "▁▁█▁"},
		{"all zero", []float64{0, 0, 0}, 10, "▁▁▁"},
		{"empty", nil, 10, ""},
		{"no room", []float64{1}, 0, ""},
	}
	for _, tc := range cases {
		if got := Sparkline(tc.values, tc.width); got != tc.want {
			t.Errorf("%s: Sparkline = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestBarLen(t *testing.T) {
	cases := []struct {
		value, max float64
		width      int
		want       int
	}{
		{5, 10, 20, 10},
		{0.1, 10, 20, 1},
		{0, 10, 20, 0},
		{10, 10, 20, 20},
		{3, 0, 20, 0},
	}
	for _, tc := range cases {
		if got := BarLen(tc.value, tc.max, tc.width); got != tc.want {
			t.Errorf("BarLen(%v, %v, %d) = %d, want %d", tc.value, tc.max, tc.width, got, tc.want)
		}
	}
}
