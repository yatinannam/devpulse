package main

import (
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
	if top < 0 { return fmt.Errorf("--top must be zero or greater") }
	session, err := traffic.LoadSession(sessionPath)
	if err != nil { return fmt.Errorf("load session: %w", err) }
	result := report.Build(session, report.Options{Title: title, SessionPath: sessionPath, Top: top, GeneratedAt: time.Now()})
	if outputPath == "-" { return report.Render(stdout, result) }
	var content strings.Builder
	if err := report.Render(&content, result); err != nil { return err }
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
	out := fs.String("out", "devpulse-report.md", "Markdown report path, or '-' for stdout")
	title := fs.String("title", "DevPulse Traffic Report", "report heading")
	top := fs.Int("top", 5, "number of slowest requests to include (0 disables)")
	_ = fs.Parse(args)
	if err := runReport(*from, *out, *title, *top, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	if *out != "-" { fmt.Printf("Report saved: %s\n", *out) }
}
