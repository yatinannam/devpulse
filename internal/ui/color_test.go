package ui

import "testing"

func lookupFrom(vars map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	}
}

func TestDecideColor(t *testing.T) {
	vtOK := func() bool { return true }
	vtFails := func() bool { return false }
	cases := []struct {
		name string
		vars map[string]string
		tty  bool
		vt   func() bool
		want bool
	}{
		{"terminal", nil, true, vtOK, true},
		{"pipe or file", nil, false, vtOK, false},
		{"NO_COLOR set", map[string]string{"NO_COLOR": "1"}, true, vtOK, false},
		{"NO_COLOR set but empty", map[string]string{"NO_COLOR": ""}, true, vtOK, false},
		{"dumb terminal", map[string]string{"TERM": "dumb"}, true, vtOK, false},
		{"virtual terminal unavailable", nil, true, vtFails, false},
		{"forced on beats NO_COLOR and pipes", map[string]string{"DEVPULSE_COLOR": "always", "NO_COLOR": "1"}, false, vtFails, true},
		{"forced off", map[string]string{"DEVPULSE_COLOR": "never"}, true, vtOK, false},
		{"forced values ignore case", map[string]string{"DEVPULSE_COLOR": " Never "}, true, vtOK, false},
		{"unknown value falls through", map[string]string{"DEVPULSE_COLOR": "auto"}, true, vtOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideColor(lookupFrom(tc.vars), tc.tty, tc.vt); got != tc.want {
				t.Fatalf("decideColor = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStylesReturnPlainTextWhenColorIsOff(t *testing.T) {
	SetColor(false)
	for name, style := range map[string]Style{"ok": OK, "warn": Warn, "error": Error, "muted": Muted, "bold": Bold, "accent": Accent, "normal": Normal} {
		if got := style("text"); got != "text" {
			t.Errorf("%s(%q) = %q with color off", name, "text", got)
		}
	}
}

func TestStylesUseBaseANSIColors(t *testing.T) {
	SetColor(true)
	defer SetColor(false)
	cases := []struct {
		name  string
		style Style
		want  string
	}{
		{"ok", OK, "\x1b[32mx\x1b[0m"},
		{"warn", Warn, "\x1b[33mx\x1b[0m"},
		{"error", Error, "\x1b[31mx\x1b[0m"},
		{"muted", Muted, "\x1b[90mx\x1b[0m"},
		{"bold", Bold, "\x1b[1mx\x1b[0m"},
		{"accent", Accent, "\x1b[35mx\x1b[0m"},
		{"normal", Normal, "x"},
	}
	for _, tc := range cases {
		if got := tc.style("x"); got != tc.want {
			t.Errorf("%s(x) = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := Error(""); got != "" {
		t.Errorf("styling empty text should add no escapes, got %q", got)
	}
}

func TestResolveWidth(t *testing.T) {
	detected := func(n int, ok bool) func() (int, bool) {
		return func() (int, bool) { return n, ok }
	}
	cases := []struct {
		name    string
		columns string
		detect  func() (int, bool)
		want    int
	}{
		{"COLUMNS wins over detection", "120", detected(90, true), 120},
		{"detected terminal width", "", detected(90, true), 90},
		{"fallback when nothing is known", "", detected(0, false), 100},
		{"invalid COLUMNS is ignored", "wide", detected(90, true), 90},
		{"clamped to the minimum", "20", detected(0, false), 60},
		{"clamped to the maximum", "400", detected(0, false), 160},
		{"detected width is clamped too", "", detected(300, true), 160},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveWidth(tc.columns, tc.detect); got != tc.want {
				t.Fatalf("resolveWidth(%q) = %d, want %d", tc.columns, got, tc.want)
			}
		})
	}
}
