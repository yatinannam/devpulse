package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/yatinannam/devpulse/internal/anomaly"
	"github.com/yatinannam/devpulse/internal/baseline"
	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

type anomalyPayload struct {
	Baseline string            `json:"baseline"`
	Session  string            `json:"session"`
	Findings []anomaly.Finding `json:"findings"`
}

func anomaliesCommand(args []string) {
	fs := flag.NewFlagSet("anomalies", flag.ExitOnError)
	baselinePath := fs.String("baseline", baseline.Path("."), "baseline snapshot to compare against")
	sessionPathFlag := fs.String("session", sessionPath(), "current traffic session")
	jsonOutput := fs.Bool("json", false, "print findings as JSON")
	mustParseArgs(fs, args)

	base, err := baseline.Load(*baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	currentSession, err := traffic.LoadSession(*sessionPathFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	services, err := discovery.Discover(300 * time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	current := baseline.Build(services, currentSession.Requests)
	findings := anomaly.Compare(base, current)

	if *jsonOutput {
		if err := output.WriteJSON(os.Stdout, anomalyPayload{
			Baseline: *baselinePath,
			Session:  *sessionPathFlag,
			Findings: findings,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		return
	}

	renderAnomalies(os.Stdout, ui.TermWidth(), *sessionPathFlag, *baselinePath, findings)
}

// renderAnomalies writes one line per finding with a glyph by severity
// (✖ error, ▲ warning, ○ info), then the counts.
func renderAnomalies(w io.Writer, width int, sessionPath, baselinePath string, findings []anomaly.Finding) {
	ui.Title(w, width, "anomalies", comparisonContext(sessionPath, baselinePath))
	if len(findings) == 0 {
		ui.Marked(w, width, ui.LevelOK, ui.Plain("No regressions against the baseline."))
		return
	}
	rows := make([]changeRow, len(findings))
	errors, warnings, infos := 0, 0, 0
	for i, f := range findings {
		level := ui.LevelIdle
		switch f.Severity {
		case anomaly.SeverityError:
			level = ui.LevelError
			errors++
		case anomaly.SeverityWarning:
			level = ui.LevelWarn
			warnings++
		default:
			infos++
		}
		rows[i] = changeRow{level, f.Kind, f.Subject, f.Message}
	}
	renderChanges(w, width, rows)
	fmt.Fprintln(w)
	ui.Writeln(w, countsLine(errors, warnings, infos))
}
