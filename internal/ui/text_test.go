package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWidthCountsRunesAndIgnoresEscapes(t *testing.T) {
	cases := map[string]int{
		"":                          0,
		"✖ :3000 …":                 9,
		"▁▂▃▄▅▆▇█":                  8,
		"\x1b[31m✖\x1b[0m ok":       4,
		"\x1b[1mdevpulse\x1b[0m st": 11,
	}
	for in, want := range cases {
		if got := Width(in); got != want {
			t.Errorf("Width(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"SamsungFindWindowsService", 10, "SamsungFi…"},
		{"node", 10, "node"},
		{"node", 4, "node"},
		{"nodejs", 1, "…"},
		{"nodejs", 0, ""},
		{"/api/a b", 7, "/api/a…"},
	}
	for _, tc := range cases {
		if got := Truncate(tc.in, tc.n); got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestTruncatePathKeepsTheEnd(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{`C:\Users\dev\projects\my-app\.devpulse\config.json`, 29, `…my-app\.devpulse\config.json`},
		{"/home/dev/.devpulse/config.json", 40, "/home/dev/.devpulse/config.json"},
		{"/a/b", 1, "…"},
		{"/a/b", 0, ""},
	}
	for _, tc := range cases {
		if got := TruncatePath(tc.in, tc.n); got != tc.want {
			t.Errorf("TruncatePath(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestWrap(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  []string
	}{
		{"Inspect the failing endpoints and response status", 20, []string{"Inspect the failing", "endpoints and", "response status"}},
		{"/api/reports/annual/2026", 10, []string{"/api/repor", "ts/annual/", "2026"}},
		{"short", 20, []string{"short"}},
		{"", 20, []string{""}},
		{"one\ntwo", 20, []string{"one", "two"}},
	}
	for _, tc := range cases {
		if got := Wrap(tc.in, tc.width); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Wrap(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}

func TestPlainAndStyledReplaceControlCharacters(t *testing.T) {
	// Paths, methods and process names come from session files and the OS.
	// A raw ESC could clear the screen or retitle the window, and a newline
	// splits a table row in two, so C0 controls, DEL and C1 controls are
	// drawn as U+FFFD instead.
	in := "/a\x1b[2J\x07\n\t\r\x00\x7f\u0085\u009bz"
	want := "/a�[2J" + strings.Repeat("�", 8) + "z"
	if got := Plain(in).Plain(); got != want {
		t.Errorf("Plain(%q) = %q, want %q", in, got, want)
	}
	if got := Styled(in, Error).Plain(); got != want {
		t.Errorf("Styled(%q) = %q, want %q", in, got, want)
	}
	if got := Truncate(in, 40); got != want {
		t.Errorf("Truncate(%q) = %q, want %q", in, got, want)
	}
	printable := "/日本語/😀 ok… ▲"
	if got := Plain(printable).Plain(); got != printable {
		t.Errorf("printable text changed: %q, want %q", got, printable)
	}
}

func TestLineTruncateKeepsStyles(t *testing.T) {
	SetColor(true)
	defer SetColor(false)
	line := Join(Plain("abc"), Styled("defgh", Error))
	got := line.Truncate(6)
	if got.Plain() != "abcde…" {
		t.Fatalf("plain = %q, want %q", got.Plain(), "abcde…")
	}
	if want := "abc" + Error("de…"); got.String() != want {
		t.Fatalf("styled = %q, want %q", got.String(), want)
	}
	if line.Plain() != "abcdefgh" {
		t.Fatalf("Truncate modified its receiver: %q", line.Plain())
	}
}

func TestLineWrapKeepsStyles(t *testing.T) {
	SetColor(true)
	defer SetColor(false)
	line := Join(Plain("12 requests "), Styled("2 errors", Error))
	got := line.Wrap(12)
	if len(got) != 2 || got[0].String() != "12 requests" || got[1].String() != Error("2 errors") {
		t.Fatalf("Wrap = %q", got)
	}
}

func TestDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                         "0ms",
		400 * time.Microsecond:    "<1ms",
		52 * time.Millisecond:     "52ms",
		999700 * time.Microsecond: "1s",
		1060 * time.Millisecond:   "1.06s",
		1300 * time.Millisecond:   "1.3s",
		65 * time.Second:          "1m5s",
	}
	for in, want := range cases {
		if got := Duration(in); got != want {
			t.Errorf("Duration(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestStatusAndLatencyStyles(t *testing.T) {
	SetColor(true)
	defer SetColor(false)
	cases := []struct {
		name, got, want string
	}{
		{"2xx", StatusStyle(200)("x"), OK("x")},
		{"3xx", StatusStyle(302)("x"), "x"},
		{"4xx", StatusStyle(404)("x"), Warn("x")},
		{"5xx", StatusStyle(503)("x"), Error("x")},
		{"fast", LatencyStyle(499 * time.Millisecond)("x"), "x"},
		{"slow", LatencyStyle(500 * time.Millisecond)("x"), Warn("x")},
		{"very slow", LatencyStyle(time.Second)("x"), Error("x")},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestLevelGlyphs(t *testing.T) {
	SetColor(false)
	want := map[Level]string{LevelOK: "●", LevelWarn: "▲", LevelError: "✖", LevelIdle: "○"}
	for level, glyph := range want {
		if got := level.Glyph().Plain(); got != glyph {
			t.Errorf("level %d glyph = %q, want %q", level, got, glyph)
		}
	}
}

func TestCount(t *testing.T) {
	if got := Count(1, "error", "errors"); got != "1 error" {
		t.Errorf("Count(1) = %q", got)
	}
	if got := Count(3, "error", "errors"); got != "3 errors" {
		t.Errorf("Count(3) = %q", got)
	}
	if got := Count(0, "error", "errors"); got != "0 errors" {
		t.Errorf("Count(0) = %q", got)
	}
}
