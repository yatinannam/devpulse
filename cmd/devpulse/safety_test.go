package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runMain runs main() with args in a child test process, so tests can check
// paths that end in os.Exit. The session and configuration point at missing
// files in a temporary directory unless env overrides them (later entries
// win). It returns stdout, stderr and the exit code.
func runMain(t *testing.T, env []string, args ...string) (string, string, int) {
	t.Helper()
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunMainHelper$")
	cmd.Env = append(os.Environ(),
		"DEVPULSE_RUN_MAIN="+string(encoded),
		"DEVPULSE_SESSION="+filepath.Join(tmp, "session.json"),
		"DEVPULSE_CONFIG="+filepath.Join(tmp, "config.json"),
	)
	cmd.Env = append(cmd.Env, env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

// TestRunMainHelper is the child side of runMain. It does nothing in a
// normal test run.
func TestRunMainHelper(t *testing.T) {
	encoded := os.Getenv("DEVPULSE_RUN_MAIN")
	if encoded == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(encoded), &args); err != nil {
		os.Exit(99)
	}
	os.Args = append([]string{"devpulse"}, args...)
	main()
	os.Exit(0)
}

func TestVersionAndHelpRejectExtraArguments(t *testing.T) {
	// README: a stray positional argument is rejected with exit status 2
	// and never silently ignored. version and help ignored theirs.
	for _, args := range [][]string{
		{"version", "extra"}, {"-v", "extra"}, {"--version", "extra"},
		{"help", "report"}, {"-h", "extra"}, {"--help", "extra"},
	} {
		stdout, stderr, code := runMain(t, nil, args...)
		if code != 2 || !strings.Contains(stderr, "unexpected argument \""+args[1]+"\"") || stdout != "" {
			t.Errorf("devpulse %s: exit %d, stdout %q, stderr %q; want exit 2 and an unexpected argument error", strings.Join(args, " "), code, stdout, stderr)
		}
	}
	if stdout, _, code := runMain(t, nil, "version"); code != 0 || stdout != "devpulse dev\n" {
		t.Errorf("devpulse version: exit %d, stdout %q", code, stdout)
	}
	if stdout, _, code := runMain(t, nil, "help"); code != 0 || !strings.Contains(stdout, "devpulse <command> [flags]") {
		t.Errorf("devpulse help: exit %d, stdout %q", code, stdout)
	}
}

func TestBaselineRefusesToOverwriteItsSession(t *testing.T) {
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
			_, stderr, code := runMain(t, nil, "baseline", "--from", sessionPath, "--to", target)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2; stderr: %s", code, stderr)
			}
			if !strings.Contains(stderr, "must differ from the source session") {
				t.Fatalf("stderr should explain the refusal: %q", stderr)
			}
			after, err := os.ReadFile(sessionPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, original) {
				t.Fatal("baseline overwrote its source session")
			}
		})
	}
}

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
