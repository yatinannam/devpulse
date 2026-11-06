package doctor

import (
	"strings"
	"testing"

	"github.com/yatinannam/devpulse/internal/ui"
)

var printFindings = []Finding{
	{"ERROR", "HTTP errors", "3 of 12 requests met the profile's error threshold (status >= 400)",
		"Inspect the failing endpoints and response status; repeated 401/403 responses often indicate authentication or session issues."},
	{"WARN", "Slow requests", "3 request(s) took 500ms or more",
		"Check backend logs and downstream calls for the slow endpoint before optimizing the client."},
}

func TestPrintWritesOneBlockPerFinding(t *testing.T) {
	ui.SetColor(false)
	var b strings.Builder
	Print(&b, printFindings, 12, 80)
	want := "" +
		"✖ HTTP errors\n" +
		"  3 of 12 requests met the profile's error threshold (status >= 400)\n" +
		"  Inspect the failing endpoints and response status; repeated 401/403 responses\n" +
		"  often indicate authentication or session issues.\n" +
		"\n" +
		"▲ Slow requests\n" +
		"  3 request(s) took 500ms or more\n" +
		"  Check backend logs and downstream calls for the slow endpoint before\n" +
		"  optimizing the client.\n" +
		"\n" +
		"1 error, 1 warning\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPrintFitsNarrowTerminals(t *testing.T) {
	ui.SetColor(false)
	var b strings.Builder
	Print(&b, printFindings, 12, 60)
	for _, line := range strings.Split(b.String(), "\n") {
		if ui.Width(line) > 60 {
			t.Errorf("line exceeds 60 columns: %q", line)
		}
	}
}

func TestPrintStylesLevelsAndCounts(t *testing.T) {
	ui.SetColor(true)
	defer ui.SetColor(false)
	var b strings.Builder
	Print(&b, append(printFindings, Finding{"WARN", "Duplicate requests", "GET /api/users called 4 times in 1.1s", "Check effects."}), 12, 80)
	out := b.String()
	for _, want := range []string{ui.Error("✖") + " " + ui.Bold("HTTP errors"), ui.Warn("▲") + " " + ui.Bold("Slow requests"), ui.Muted("Check effects."), ui.Error("1 error") + ", " + ui.Warn("2 warnings")} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestPrintWithoutFindings(t *testing.T) {
	ui.SetColor(false)
	var b strings.Builder
	Print(&b, []Finding{}, 12, 80)
	if want := "● No issues found in 12 requests.\n"; b.String() != want {
		t.Fatalf("got %q, want %q", b.String(), want)
	}
}
