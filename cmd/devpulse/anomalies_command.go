package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yatinannam/devpulse/internal/anomaly"
	"github.com/yatinannam/devpulse/internal/baseline"
	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/traffic"
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
	_ = fs.Parse(args)

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

	fmt.Println("DEVPULSE ANOMALIES")
	fmt.Println("------------------------------------------------")
	fmt.Printf("Baseline: %s\nSession:  %s\n", *baselinePath, *sessionPathFlag)
	if len(findings) == 0 {
		fmt.Println()
		fmt.Println("No baseline regressions detected.")
		return
	}
	for _, finding := range findings {
		fmt.Printf("[%s] %-18s %s: %s\n",
			finding.Severity, finding.Kind, finding.Subject, finding.Message)
	}
}
