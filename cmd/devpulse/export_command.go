package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

type exportOptions struct {
	Method     string
	Status     int
	PathPrefix string
	Since      time.Time
	Until      time.Time
}

func runExport(sessionPath, outputPath string, opts exportOptions, stdout io.Writer) error {
	session, err := traffic.LoadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if !opts.Since.IsZero() && !opts.Until.IsZero() && opts.Since.After(opts.Until) {
		return fmt.Errorf("--since must not be after --until")
	}

	var content bytes.Buffer
	writer := csv.NewWriter(&content)
	if err := writer.Write([]string{"timestamp", "method", "path", "status", "latency_ms", "bytes", "target_host", "target_port"}); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, entry := range session.Requests {
		record := []string{
			entry.Time.UTC().Format(time.RFC3339Nano),
			entry.Method,
			entry.Path,
			strconv.Itoa(entry.Status),
			strconv.FormatFloat(float64(entry.Latency)/float64(time.Millisecond), 'f', 3, 64),
			strconv.FormatInt(entry.Bytes, 10),
			entry.TargetHost,
			strconv.Itoa(entry.TargetPort),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV output: %w", err)
	}

	if outputPath == "" || outputPath == "-" {
		if _, err := stdout.Write(content.Bytes()); err != nil {
			return fmt.Errorf("write CSV to stdout: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, content.Bytes(), 0600); err != nil {
		return fmt.Errorf("write CSV file: %w", err)
	}
	return nil
}

func exportCommand(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to export")
	out := fs.String("out", "-", "CSV output path, or '-' for stdout")
	method := fs.String("method", "", "filter by HTTP method")
	statusCode := fs.Int("status", 0, "filter by HTTP status code")
	pathPrefix := fs.String("path-prefix", "", "filter paths by prefix")
	sinceText := fs.String("since", "", "include requests at or after RFC3339 timestamp")
	untilText := fs.String("until", "", "include requests at or before RFC3339 timestamp")
	_ = fs.Parse(args)

	since, err := parseExportTime("--since", *sinceText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(2)
	}
	until, err := parseExportTime("--until", *untilText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(2)
	}
	if *statusCode != 0 && (*statusCode < 100 || *statusCode > 599) {
		fmt.Fprintln(os.Stderr, "devpulse: --status must be between 100 and 599")
		os.Exit(2)
	}
	if err := runExport(*from, *out, exportOptions{
		Method: *method, Status: *statusCode, PathPrefix: *pathPrefix, Since: since, Until: until,
	}, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	if *out != "-" && *out != "" {
		fmt.Printf("CSV exported: %s\n", *out)
	}
}

func parseExportTime(flagName, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be an RFC3339 timestamp: %w", flagName, err)
	}
	return parsed, nil
}
