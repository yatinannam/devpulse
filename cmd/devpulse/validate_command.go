package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/sessioncheck"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
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
	if s, err = traffic.DecodeSession(data); err != nil {
		return 1, fmt.Errorf("decode session: %w", err)
	}
	report := sessioncheck.Validate(s)
	if jsonOutput {
		if err := output.WriteJSON(out, validatePayload{Session: path, Report: report}); err != nil {
			return 1, err
		}
	} else {
		renderValidation(out, ui.TermWidth(), path, report)
	}
	if !report.Valid {
		return 1, nil
	}
	return 0, nil
}

// renderValidation writes the verdict, then a Request, Field, Code,
// Message table of the issues. Session-level issues show "-" as the
// request.
func renderValidation(w io.Writer, width int, path string, report sessioncheck.Report) {
	ui.Title(w, width, "validate", sessionContext(path, report.RequestCount))
	if report.Valid {
		ui.Marked(w, width, ui.LevelOK, ui.Plain(fmt.Sprintf("Session is valid (%s).", ui.Count(report.RequestCount, "request", "requests"))))
		return
	}
	ui.Marked(w, width, ui.LevelError, ui.Plain("Session is invalid: "+ui.Count(len(report.Issues), "issue", "issues")))
	fmt.Fprintln(w)
	table := ui.NewTable(
		ui.Column{Header: "Request", Align: ui.AlignRight},
		ui.Column{Header: "Field"},
		ui.Column{Header: "Code"},
		ui.Column{Header: "Message", Flex: true, Wrap: true, Min: 20},
	)
	for _, issue := range report.Issues {
		request := ui.Styled("-", ui.Muted)
		if issue.Request != 0 {
			request = ui.Plain(strconv.Itoa(issue.Request))
		}
		table.Row(request, ui.Plain(issue.Field), ui.Plain(issue.Code), ui.Plain(issue.Message))
	}
	table.Render(w, width)
}

func validateCommand(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to validate")
	jsonOutput := fs.Bool("json", false, "print validation report as JSON")
	mustParseArgs(fs, args)

	code, err := runValidate(*from, *jsonOutput, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	if code != 0 {
		os.Exit(code)
	}
}
