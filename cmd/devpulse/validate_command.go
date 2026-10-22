package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/sessioncheck"
	"github.com/yatinannam/devpulse/internal/traffic"
)

type validatePayload struct {
	Session string              `json:"session"`
	Report  sessioncheck.Report `json:"report"`
}

func runValidate(path string, jsonOutput bool, out io.Writer) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 1, fmt.Errorf("read session: %w", err)
	}
	var s traffic.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return 1, fmt.Errorf("decode session: %w", err)
	}
	report := sessioncheck.Validate(s)
	if jsonOutput {
		if err := output.WriteJSON(out, validatePayload{Session: path, Report: report}); err != nil {
			return 1, err
		}
	} else {
		fmt.Fprintln(out, "DEVPULSE VALIDATE")
		fmt.Fprintln(out, "------------------------------------------------")
		fmt.Fprintf(out, "Session: %s\nRequests: %d\n", path, report.RequestCount)
		if report.Valid {
			fmt.Fprintln(out, "Session is valid.")
		} else {
			fmt.Fprintf(out, "Session is invalid (%d issue(s)).\n", len(report.Issues))
			for _, issue := range report.Issues {
				if issue.Request == 0 {
					fmt.Fprintf(out, "[%s] %s: %s\n", issue.Code, issue.Field, issue.Message)
				} else {
					fmt.Fprintf(out, "[request %d] %s (%s): %s\n", issue.Request, issue.Field, issue.Code, issue.Message)
				}
			}
		}
	}
	if !report.Valid {
		return 1, nil
	}
	return 0, nil
}

func validateCommand(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to validate")
	jsonOutput := fs.Bool("json", false, "print validation report as JSON")
	_ = fs.Parse(args)

	code, err := runValidate(*from, *jsonOutput, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	if code != 0 {
		os.Exit(code)
	}
}
