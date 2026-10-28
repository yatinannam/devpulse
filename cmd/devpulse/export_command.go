package main

import (
	"bytes"
	"flag"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

type exportOptions struct {
	Method     string
	Status     int
	PathPrefix string
	Since      time.Time
	Until      time.Time
	Limit      int
}

func runExport(sessionPath, outputPath string, opts exportOptions, stdout io.Writer) error {
	session, err := traffic.LoadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if !opts.Since.IsZero() && !opts.Until.IsZero() && opts.Since.After(opts.Until) {
		return fmt.Errorf("--since must not be after --until")
	}
	if opts.Status != 0 && (opts.Status < 100 || opts.Status > 599) {
		return fmt.Errorf("--status must be between 100 and 599")
	}
	if opts.Limit < 0 {
		return fmt.Errorf("--limit must be zero or greater")
	}
	requests := filterExportRequests(session.Requests, opts)
	if opts.Limit > 0 && len(requests) > opts.Limit {
		requests = requests[:opts.Limit]
	}

	var content bytes.Buffer
	writer := csv.NewWriter(&content)
	if err := writer.Write([]string{"timestamp", "method", "path", "status", "latency_ms", "bytes", "target_host", "target_port"}); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, entry := range requests {
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
	inputAbs, err := filepath.Abs(sessionPath)
	if err != nil {
		return fmt.Errorf("resolve session path: %w", err)
	}
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if inputAbs == outputAbs {
		return fmt.Errorf("output path must differ from the source session")
	}
	if err := writeExportFile(outputPath, content.Bytes()); err != nil {
		return err
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
	limit := fs.Int("limit", 0, "maximum number of matching requests to export (0 means unlimited)")
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
		Method: *method, Status: *statusCode, PathPrefix: *pathPrefix, Since: since, Until: until, Limit: *limit,
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


// filterExportRequests applies the optional filters without mutating the source session.
func filterExportRequests(entries []traffic.Request, opts exportOptions) []traffic.Request {
	method := strings.ToUpper(strings.TrimSpace(opts.Method))
	out := make([]traffic.Request, 0, len(entries))
	for _, entry := range entries {
		if method != "" && !strings.EqualFold(strings.TrimSpace(entry.Method), method) {
			continue
		}
		if opts.Status != 0 && entry.Status != opts.Status {
			continue
		}
		if opts.PathPrefix != "" && !strings.HasPrefix(entry.Path, opts.PathPrefix) {
			continue
		}
		if !opts.Since.IsZero() && entry.Time.Before(opts.Since) {
			continue
		}
		if !opts.Until.IsZero() && entry.Time.After(opts.Until) {
			continue
		}
		out = append(out, entry)
	}
	return out
}


// writeExportFile replaces a CSV file atomically and keeps its contents private.
func writeExportFile(outputPath string, data []byte) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".devpulse-export-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary CSV file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary CSV file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary CSV file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary CSV file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary CSV file: %w", err)
	}
	if err := os.Rename(tmpName, outputPath); err != nil {
		return fmt.Errorf("replace CSV output: %w", err)
	}
	return nil
}
