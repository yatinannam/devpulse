package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yatinannam/devpulse/internal/report"
	"github.com/yatinannam/devpulse/internal/traffic"
)

func runReport(sessionPath, outputPath, title string, top int, stdout io.Writer) error {
	return runReportWithFormat(sessionPath, outputPath, title, top, "markdown", stdout)
}

func runReportWithFormat(sessionPath, outputPath, title string, top int, format string, stdout io.Writer) error {
	if top < 0 {
		return fmt.Errorf("--top must be zero or greater")
	}
	if format != "markdown" && format != "json" {
		return fmt.Errorf("unsupported report format %q (choose markdown or json)", format)
	}
	session, err := traffic.LoadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	result := report.Build(session, report.Options{Title: title, SessionPath: sessionPath, Top: top, GeneratedAt: time.Now()})
	var content strings.Builder
	if format == "json" {
		encoder := json.NewEncoder(&content)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("encode JSON report: %w", err)
		}
	} else if err := report.Render(&content, result); err != nil {
		return err
	}
	if isStdoutPath(outputPath) {
		_, err := io.WriteString(stdout, content.String())
		return err
	}
	if err := ensureDistinctOutput(sessionPath, outputPath); err != nil {
		return err
	}
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".devpulse-report-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary report: %w", err)
	}
	if _, err := io.WriteString(tmp, content.String()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write report: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close report: %w", err)
	}
	if err := os.Rename(tmpName, outputPath); err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	return nil
}

// reportArgs holds the parsed and resolved `devpulse report` flags.
type reportArgs struct {
	From, Out, Format, Title string
	Top                      int
}

// parseReportArgs defines the report flags on fs, parses args, and resolves
// the output path. An explicit --out always wins, even when it equals the
// Markdown default; without one, --format json writes devpulse-report.json.
// An empty --out means stdout, like --out -.
func parseReportArgs(fs *flag.FlagSet, args []string) (reportArgs, error) {
	var a reportArgs
	fs.StringVar(&a.From, "from", sessionPath(), "traffic session to report on")
	fs.StringVar(&a.Out, "out", "devpulse-report.md", "report output path, or '-' for stdout")
	fs.StringVar(&a.Format, "format", "markdown", "report format: markdown or json")
	fs.StringVar(&a.Title, "title", "DevPulse Traffic Report", "report heading")
	fs.IntVar(&a.Top, "top", 5, "number of slowest requests to include (0 disables)")
	if err := parseArgs(fs, args); err != nil {
		return a, err
	}
	// Validate here so bad values are usage errors (exit 2) and are reported
	// before any session is loaded. runReportWithFormat keeps its own checks
	// for library callers.
	if a.Top < 0 {
		return a, fmt.Errorf("--top must be zero or greater")
	}
	if a.Format != "markdown" && a.Format != "json" {
		return a, fmt.Errorf("unsupported report format %q (choose markdown or json)", a.Format)
	}
	outSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "out" {
			outSet = true
		}
	})
	if !outSet && a.Format == "json" {
		a.Out = "devpulse-report.json"
	}
	if isStdoutPath(a.Out) {
		a.Out = "-"
	}
	return a, nil
}

// isStdoutPath reports whether an --out value selects standard output.
func isStdoutPath(path string) bool {
	return path == "" || path == "-"
}

func reportCommand(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	opts, err := parseReportArgs(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse %s: %v\n", fs.Name(), err)
		os.Exit(2)
	}
	if err := runReportWithFormat(opts.From, opts.Out, opts.Title, opts.Top, opts.Format, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	if !isStdoutPath(opts.Out) {
		fmt.Printf("Report saved: %s\n", opts.Out)
	}
}
