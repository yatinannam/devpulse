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
	if top < 0 { return fmt.Errorf("--top must be zero or greater") }
	if format != "markdown" && format != "json" { return fmt.Errorf("unsupported report format %q (choose markdown or json)", format) }
	session, err := traffic.LoadSession(sessionPath)
	if err != nil { return fmt.Errorf("load session: %w", err) }
	result := report.Build(session, report.Options{Title: title, SessionPath: sessionPath, Top: top, GeneratedAt: time.Now()})
	var content strings.Builder
	if format == "json" {
		encoder := json.NewEncoder(&content)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil { return fmt.Errorf("encode JSON report: %w", err) }
	} else if err := report.Render(&content, result); err != nil { return err }
	if outputPath == "-" { _, err := io.WriteString(stdout, content.String()); return err }
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil { return fmt.Errorf("create report directory: %w", err) }
	tmp, err := os.CreateTemp(dir, ".devpulse-report-*.tmp")
	if err != nil { return fmt.Errorf("create temporary report: %w", err) }
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil { _ = tmp.Close(); return fmt.Errorf("secure temporary report: %w", err) }
	if _, err := io.WriteString(tmp, content.String()); err != nil { _ = tmp.Close(); return fmt.Errorf("write report: %w", err) }
	if err := tmp.Close(); err != nil { return fmt.Errorf("close report: %w", err) }
	if err := os.Rename(tmpName, outputPath); err != nil { return fmt.Errorf("save report: %w", err) }
	return nil
}

func reportCommand(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to report on")
	out := fs.String("out", "devpulse-report.md", "report output path, or '-' for stdout")
	format := fs.String("format", "markdown", "report format: markdown or json")
	title := fs.String("title", "DevPulse Traffic Report", "report heading")
	top := fs.Int("top", 5, "number of slowest requests to include (0 disables)")
	_ = fs.Parse(args)
	if *format == "json" && *out == "devpulse-report.md" { *out = "devpulse-report.json" }
	if err := runReportWithFormat(*from, *out, *title, *top, *format, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	if *out != "-" { fmt.Printf("Report saved: %s\n", *out) }
}
