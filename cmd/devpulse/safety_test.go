package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sessionAliases returns paths that name the same file as sessionPath but
// differ as strings, so a plain path comparison cannot detect them.
func sessionAliases(t *testing.T, sessionPath string) map[string]string {
	t.Helper()
	aliases := map[string]string{}
	link := filepath.Join(filepath.Dir(sessionPath), "linked-session.json")
	if err := os.Link(sessionPath, link); err == nil {
		aliases["hard link"] = link
	} else {
		t.Logf("hard links unavailable: %v", err)
	}
	if runtime.GOOS == "windows" {
		aliases["case variant"] = filepath.Join(filepath.Dir(sessionPath), strings.ToUpper(filepath.Base(sessionPath)))
	}
	if len(aliases) == 0 {
		t.Skip("no way to alias a file on this platform")
	}
	return aliases
}

func TestRunExportRejectsOutputThatAliasesTheSession(t *testing.T) {
	sessionPath := writeExportFixture(t)
	original, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, alias := range sessionAliases(t, sessionPath) {
		t.Run(name, func(t *testing.T) {
			if err := runExport(sessionPath, alias, exportOptions{}, &bytes.Buffer{}); err == nil {
				t.Fatalf("expected export to %s to be rejected", alias)
			}
			after, err := os.ReadFile(sessionPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, original) {
				t.Fatal("source session was modified")
			}
		})
	}
}

func TestRunReportRejectsOutputThatIsTheSession(t *testing.T) {
	sessionPath := writeReportFixture(t)
	original, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[string]string{"same path": sessionPath}
	for name, alias := range sessionAliases(t, sessionPath) {
		targets[name] = alias
	}
	for name, target := range targets {
		t.Run(name, func(t *testing.T) {
			for _, format := range []string{"markdown", "json"} {
				if err := runReportWithFormat(sessionPath, target, "Report", 1, format, &bytes.Buffer{}); err == nil {
					t.Fatalf("expected %s report to %s to be rejected", format, target)
				}
			}
			after, err := os.ReadFile(sessionPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, original) {
				t.Fatal("source session was modified")
			}
		})
	}
}

func TestParseArgsRejectsPositionalArguments(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"flags only", []string{"--from", "session.json"}, false},
		{"no args", nil, false},
		{"bare path", []string{"./ci/session.json"}, true},
		{"flag after positional", []string{"session.json", "--out", "x.csv"}, true},
		{"trailing positional", []string{"--from", "a.json", "extra"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			fs.String("from", "", "")
			fs.String("out", "", "")
			err := parseArgs(fs, tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseArgs(%q) error = %v, wantErr %v", tc.args, err, tc.wantErr)
			}
		})
	}
}

func TestParseWatchInterval(t *testing.T) {
	for _, bad := range []string{"0s", "-1s", "abc", ""} {
		if _, err := parseWatchInterval(bad); err == nil {
			t.Errorf("parseWatchInterval(%q) should fail", bad)
		}
	}
	if got, err := parseWatchInterval("5s"); err != nil || got.Seconds() != 5 {
		t.Errorf("parseWatchInterval(5s) = %v, %v", got, err)
	}
}
