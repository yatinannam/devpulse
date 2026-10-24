package report

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Render writes a Markdown report to w.
func Render(w io.Writer, r Report) error {
	if r.Title == "" { r.Title = "DevPulse Traffic Report" }
	if _, err := fmt.Fprintf(w, "# %s\n\n", r.Title); err != nil { return err }
	if _, err := fmt.Fprintf(w, "- Session: %s\n- Session created: %s\n- Report generated: %s\n\n",
		r.SessionPath, formatTime(r.SessionCreatedAt), formatTime(r.GeneratedAt)); err != nil { return err }
	if _, err := fmt.Fprint(w, "## Summary\n\n| Metric | Value |\n| --- | ---: |\n"); err != nil { return err }
	rows := [][2]string{
		{"Requests", fmt.Sprintf("%d", r.Summary.Total)},
		{"Errors", fmt.Sprintf("%d", r.Summary.Errors)},
		{"Error rate", fmt.Sprintf("%.1f%%", r.Summary.ErrorRate*100)},
		{"Average latency", r.Summary.Average.Round(time.Millisecond).String()},
		{"P50 latency", r.Summary.P50.Round(time.Millisecond).String()},
		{"P95 latency", r.Summary.P95.Round(time.Millisecond).String()},
		{"P99 latency", r.Summary.P99.Round(time.Millisecond).String()},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "| %s | %s |\n", row[0], row[1]); err != nil { return err }
	}
	if _, err := fmt.Fprint(w, "\n## Top Endpoints\n\n| Method | Path | Requests | Errors | Error rate | Average latency |\n| --- | --- | ---: | ---: | ---: | ---: |\n"); err != nil { return err }
	if len(r.Endpoints) == 0 {
		if _, err := fmt.Fprintln(w, "| — | No requests | 0 | 0 | 0.0% | 0s |"); err != nil { return err }
	} else {
		for _, endpoint := range r.Endpoints {
			if _, err := fmt.Fprintf(w, "| %s | %s | %d | %d | %.1f%% | %s |\n",
				escapeCell(endpoint.Method), escapeCell(endpoint.Path), endpoint.Requests, endpoint.Errors,
				endpoint.ErrorRate*100, endpoint.Average.Round(time.Millisecond)); err != nil { return err }
		}
	}
	if _, err := fmt.Fprint(w, "\n## Slowest Requests\n\n| # | Time | Method | Path | Status | Latency |\n| ---: | --- | --- | --- | ---: | ---: |\n"); err != nil { return err }
	if len(r.Slowest) == 0 {
		if _, err := fmt.Fprintln(w, "| — | — | — | No requests | — | — |"); err != nil { return err }
	} else {
		for i, request := range r.Slowest {
			if _, err := fmt.Fprintf(w, "| %d | %s | %s | %s | %d | %s |\n",
				i+1, formatTime(request.Time), escapeCell(request.Method), escapeCell(request.Path),
				request.Status, request.Latency.Round(time.Millisecond)); err != nil { return err }
		}
	}
	if _, err := fmt.Fprint(w, "\n## HTTP Status Codes\n\n| Status | Requests |\n| ---: | ---: |\n"); err != nil { return err }
	if len(r.StatusCodes) == 0 {
		if _, err := fmt.Fprintln(w, "| — | 0 |"); err != nil { return err }
	} else {
		for _, status := range r.StatusCodes {
			if _, err := fmt.Fprintf(w, "| %d | %d |\n", status.Status, status.Count); err != nil { return err }
		}
	}
	if _, err := fmt.Fprint(w, "\n## Diagnostic Findings\n\n"); err != nil { return err }
	if len(r.Findings) == 0 {
		if _, err := fmt.Fprintln(w, "No obvious HTTP issues detected."); err != nil { return err }
	} else {
		for _, finding := range r.Findings {
			if _, err := fmt.Fprintf(w, "- **%s — %s:** %s\n  - Recommendation: %s\n",
				escapeCell(finding.Level), escapeCell(finding.Title), escapeCell(finding.Detail), escapeCell(finding.Advice)); err != nil { return err }
		}
	}
	return nil
}

func escapeCell(s string) string {
	return strings.NewReplacer("\\", "\\\\", "|", "\\|", "\r", " ", "\n", " ").Replace(s)
}

func formatTime(t time.Time) string {
	if t.IsZero() { return "unknown" }
	return t.UTC().Format(time.RFC3339)
}
