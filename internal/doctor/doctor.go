package doctor

import (
	"fmt"
	"sort"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

type Profile string

const (
	ProfileDefault   Profile = "default"
	ProfileStrict    Profile = "strict"
	ProfilePermissive Profile = "permissive"
)

type Thresholds struct {
	SlowRequest time.Duration
	ErrorStatus int
	MinDuplicateRequests int
	DuplicateWindow time.Duration
}

func ThresholdsFor(profile Profile) (Thresholds, error) {
	switch profile {
	case "", ProfileDefault:
		return Thresholds{SlowRequest: 500 * time.Millisecond, ErrorStatus: 400, MinDuplicateRequests: 3, DuplicateWindow: 3 * time.Second}, nil
	case ProfileStrict:
		return Thresholds{SlowRequest: 250 * time.Millisecond, ErrorStatus: 400, MinDuplicateRequests: 2, DuplicateWindow: 5 * time.Second}, nil
	case ProfilePermissive:
		return Thresholds{SlowRequest: time.Second, ErrorStatus: 500, MinDuplicateRequests: 4, DuplicateWindow: 2 * time.Second}, nil
	default:
		return Thresholds{}, fmt.Errorf("unknown diagnostic profile %q (valid: default, strict, permissive)", profile)
	}
}

type Finding struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Advice string `json:"advice"`
}

func Analyze(entries []traffic.Request) []Finding {
	return AnalyzeWithProfile(entries, ProfileDefault)
}

// AnalyzeWithProfile applies an explicit, deterministic threshold profile.
func AnalyzeWithProfile(entries []traffic.Request, profile Profile) []Finding {
	thresholds, err := ThresholdsFor(profile)
	if err != nil || len(entries) == 0 {
		return nil
	}
	var findings []Finding
	var errors, slow int
	for _, e := range entries {
		if e.Status >= thresholds.ErrorStatus {
			errors++
		}
		if e.Latency >= thresholds.SlowRequest {
			slow++
		}
	}
	if errors > 0 {
		findings = append(findings, Finding{"ERROR", "HTTP errors",
			fmt.Sprintf("%d of %d requests met the profile's error threshold (status >= %d)", errors, len(entries), thresholds.ErrorStatus),
			"Inspect the failing endpoints and response status; repeated 401/403 responses often indicate authentication or session issues."})
	}
	if slow > 0 {
		findings = append(findings, Finding{"WARN", "Slow requests",
			fmt.Sprintf("%d request(s) took %s or more", slow, thresholds.SlowRequest),
			"Check backend logs and downstream calls for the slow endpoint before optimizing the client."})
	}

	type key struct{ method, path string }
	groups := map[key][]traffic.Request{}
	for _, e := range entries {
		k := key{e.Method, e.Path}
		groups[k] = append(groups[k], e)
	}
	type duplicate struct {
		k key
		count int
		first, last time.Time
	}
	var duplicates []duplicate
	for k, group := range groups {
		if len(group) < thresholds.MinDuplicateRequests {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].Time.Before(group[j].Time) })
		first, last := group[0].Time, group[len(group)-1].Time
		if last.Sub(first) <= thresholds.DuplicateWindow {
			duplicates = append(duplicates, duplicate{k, len(group), first, last})
		}
	}
	sort.Slice(duplicates, func(i, j int) bool {
		if duplicates[i].count != duplicates[j].count {
			return duplicates[i].count > duplicates[j].count
		}
		if duplicates[i].k.path != duplicates[j].k.path {
			return duplicates[i].k.path < duplicates[j].k.path
		}
		return duplicates[i].k.method < duplicates[j].k.method
	})
	for _, d := range duplicates {
		findings = append(findings, Finding{"WARN", "Duplicate requests",
			fmt.Sprintf("%s %s called %d times in %.1fs", d.k.method, d.k.path, d.count, d.last.Sub(d.first).Seconds()),
			"Check frontend effects, event handlers, retries, and data-fetching logic for an unintended request loop."})
	}
	return findings
}

func Print(findings []Finding) {
	fmt.Println("DEVPULSE DOCTOR")
	fmt.Println("------------------------------------------------")
	if len(findings) == 0 {
		fmt.Println("No obvious HTTP issues detected.")
		return
	}
	for _, f := range findings {
		fmt.Printf("%s  %s - %s\n", f.Level, f.Title, f.Detail)
		fmt.Printf("       %s\n", f.Advice)
	}
}
