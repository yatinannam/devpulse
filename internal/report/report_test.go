package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func sampleSession() traffic.Session {
	created := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	return traffic.Session{Version: 1, CreatedAt: created, Requests: []traffic.Request{
		{Time: created, Method: "GET", Path: "/health", Status: 200, Latency: 10*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
		{Time: created.Add(time.Second), Method: "GET", Path: "/api/items", Status: 500, Latency: 800*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
		{Time: created.Add(2*time.Second), Method: "GET", Path: "/api/items", Status: 200, Latency: 100*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
	}}
}

func TestBuildSummaryAndMetadata(t *testing.T) {
	generated := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	got := Build(sampleSession(), Options{Title: "CI Report", SessionPath: "session.json", Top: 2, GeneratedAt: generated})
	if got.Title != "CI Report" || got.SessionPath != "session.json" || !got.GeneratedAt.Equal(generated) {
		t.Fatalf("metadata mismatch: %+v", got)
	}
	if got.Summary.Total != 3 || got.Summary.Errors != 1 || got.Summary.ErrorRate < 0.333 || got.Summary.ErrorRate > 0.334 {
		t.Fatalf("summary mismatch: %+v", got.Summary)
	}
	if len(got.Slowest) != 2 || got.Slowest[0].Path != "/api/items" || got.Slowest[0].Latency != 800*time.Millisecond {
		t.Fatalf("slowest mismatch: %+v", got.Slowest)
	}
}

func TestRenderIncludesSections(t *testing.T) {
	got := Build(sampleSession(), Options{SessionPath: "session.json", Top: 3, GeneratedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)})
	var out bytes.Buffer
	if err := Render(&out, got); err != nil { t.Fatal(err) }
	for _, want := range []string{"# DevPulse Traffic Report", "## Summary", "## HTTP Status Codes", "## Top Endpoints", "## Slowest Requests", "## Diagnostic Findings"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) { t.Errorf("report missing %q", want) }
	}
}

func TestBuildEndpointsSortedByVolumeThenErrors(t *testing.T) {
	got := Build(sampleSession(), Options{Top: 5})
	if len(got.Endpoints) != 2 { t.Fatalf("got %d endpoints, want 2", len(got.Endpoints)) }
	if got.Endpoints[0].Path != "/api/items" || got.Endpoints[0].Requests != 2 || got.Endpoints[0].Errors != 1 {
		t.Fatalf("unexpected top endpoint: %+v", got.Endpoints[0])
	}
	if got.Endpoints[0].Average != 450*time.Millisecond || got.Endpoints[0].ErrorRate != 0.5 {
		t.Fatalf("unexpected endpoint metrics: %+v", got.Endpoints[0])
	}
}

func TestBuildEndpointOrderIsDeterministicForTies(t *testing.T) {
	session := sampleSession()
	session.Requests = append(session.Requests,
		traffic.Request{Time: session.CreatedAt, Method: "POST", Path: "/api/a", Status: 200, Latency: time.Millisecond},
		traffic.Request{Time: session.CreatedAt, Method: "GET", Path: "/api/a", Status: 200, Latency: time.Millisecond})
	got := Build(session, Options{Top: 5})
	for i := 1; i < len(got.Endpoints); i++ {
		prev, curr := got.Endpoints[i-1], got.Endpoints[i]
		if prev.Requests == curr.Requests && prev.Errors == curr.Errors && prev.Path > curr.Path {
			t.Fatalf("path order is not stable: %+v", got.Endpoints)
		}
	}
}

func TestBuildStatusCodesSortedAndCounted(t *testing.T) {
	got := Build(sampleSession(), Options{Top: 0})
	if len(got.StatusCodes) != 2 || got.StatusCodes[0] != (StatusCount{Status: 200, Count: 2}) || got.StatusCodes[1] != (StatusCount{Status: 500, Count: 1}) {
		t.Fatalf("unexpected status distribution: %+v", got.StatusCodes)
	}
	if len(got.Slowest) != 0 { t.Fatalf("Top=0 should disable slowest list, got %+v", got.Slowest) }
}

func TestBuildEmptySessionHasEmptyCollections(t *testing.T) {
	got := Build(traffic.Session{}, Options{Top: 5})
	if got.Summary.Total != 0 || len(got.StatusCodes) != 0 || len(got.Endpoints) != 0 || len(got.Slowest) != 0 || len(got.Findings) != 0 {
		t.Fatalf("unexpected empty report: %+v", got)
	}
}

func TestBuildJSONEmitsEmptyArraysNotNull(t *testing.T) {
	cases := map[string]Report{
		"top zero":      Build(sampleSession(), Options{Top: 0}),
		"empty session": Build(traffic.Session{}, Options{Top: 5}),
	}
	for name, got := range cases {
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if string(decoded["slowest_requests"]) != "[]" {
			t.Errorf("%s: slowest_requests = %s, want []", name, decoded["slowest_requests"])
		}
		if name == "empty session" {
			for _, key := range []string{"status_codes", "endpoints", "findings"} {
				if string(decoded[key]) != "[]" {
					t.Errorf("%s: %s = %s, want []", name, key, decoded[key])
				}
			}
		}
	}
}

func TestRenderEscapesMarkdownTableCells(t *testing.T) {
	session := sampleSession()
	// Requests[1] is the slowest request, so Top: 1 puts it in the Slowest
	// table as well as in Top Endpoints.
	session.Requests[1].Path = "/a|b"
	session.Requests[1].Method = "GET\nInjected"
	got := Build(session, Options{Top: 1, GeneratedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)})
	if len(got.Slowest) != 1 || got.Slowest[0].Path != "/a|b" {
		t.Fatalf("fixture no longer puts the escaped request in the Slowest table: %+v", got.Slowest)
	}
	var out bytes.Buffer
	if err := Render(&out, got); err != nil { t.Fatal(err) }
	text := out.String()
	if strings.Contains(text, "GET\nInjected") {
		t.Fatalf("newline escaped into table output: %s", text)
	}
	for _, name := range []string{"Top Endpoints", "Slowest Requests"} {
		section := markdownSection(t, text, name)
		if !strings.Contains(section, `| GET Injected | /a\|b |`) {
			t.Errorf("%s table does not escape the method and path:\n%s", name, section)
		}
	}
}

func TestRenderTitleAndSessionPathCannotInjectMarkdown(t *testing.T) {
	got := Build(sampleSession(), Options{
		Title:       "Smoke\n## Fake Title\r\n- [x](javascript:alert(1)) *bold* `code` #",
		SessionPath: "session.json\n## Fake Section\n",
		Top:         1,
		GeneratedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC),
	})
	var out bytes.Buffer
	if err := Render(&out, got); err != nil { t.Fatal(err) }
	text := out.String()
	var headings []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}
	want := []string{
		"# Smoke \\#\\# Fake Title - \\[x\\](javascript:alert(1)) \\*bold\\* \\`code\\` \\#",
		"## Summary", "## Top Endpoints", "## HTTP Status Codes", "## Slowest Requests", "## Diagnostic Findings",
	}
	if strings.Join(headings, "\n") != strings.Join(want, "\n") {
		t.Fatalf("headings = %q, want %q", headings, want)
	}
	if !strings.Contains(text, "- Session: session.json \\#\\# Fake Section \n") {
		t.Fatalf("session path was not flattened and escaped:\n%s", text)
	}
}

func TestRenderSectionsFollowDocumentedOrder(t *testing.T) {
	got := Build(sampleSession(), Options{Top: 3, GeneratedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)})
	var out bytes.Buffer
	if err := Render(&out, got); err != nil { t.Fatal(err) }
	text := out.String()
	last := -1
	for _, heading := range []string{"## Summary", "## Top Endpoints", "## HTTP Status Codes", "## Slowest Requests", "## Diagnostic Findings"} {
		i := strings.Index(text, "\n"+heading+"\n")
		if i < 0 {
			t.Fatalf("report missing %q:\n%s", heading, text)
		}
		if i < last {
			t.Fatalf("%q is out of the documented order:\n%s", heading, text)
		}
		last = i
	}
}

func TestRenderTopZeroSaysSlowestDetailsWereOmitted(t *testing.T) {
	got := Build(sampleSession(), Options{Top: 0, GeneratedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)})
	var out bytes.Buffer
	if err := Render(&out, got); err != nil { t.Fatal(err) }
	section := markdownSection(t, out.String(), "Slowest Requests")
	if strings.Contains(section, "No requests") {
		t.Fatalf("Top 0 on a non-empty session claims there are no requests:\n%s", section)
	}
	if !strings.Contains(section, "omitted with `--top 0`") {
		t.Fatalf("Top 0 does not explain the omission:\n%s", section)
	}
}

func TestRenderEmptySessionSaysNoRequests(t *testing.T) {
	got := Build(traffic.Session{}, Options{Top: 0})
	var out bytes.Buffer
	if err := Render(&out, got); err != nil { t.Fatal(err) }
	section := markdownSection(t, out.String(), "Slowest Requests")
	if !strings.Contains(section, "No requests") || strings.Contains(section, "omitted") {
		t.Fatalf("empty session should report no requests:\n%s", section)
	}
}

func TestReportJSONUsesSnakeCaseFieldNames(t *testing.T) {
	data, err := json.Marshal(Build(sampleSession(), Options{Top: 1}))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Slowest []map[string]json.RawMessage `json:"slowest_requests"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Slowest) != 1 {
		t.Fatalf("got %d slowest requests, want 1: %s", len(decoded.Slowest), data)
	}
	request := decoded.Slowest[0]
	for _, key := range []string{"time", "method", "path", "status", "latency", "bytes", "target_host", "target_port"} {
		if _, ok := request[key]; !ok {
			t.Errorf("slowest request missing %q: %s", key, data)
		}
	}
	if len(request) != 8 {
		t.Errorf("slowest request has unexpected fields: %s", data)
	}
	if string(request["target_host"]) != `"localhost"` || string(request["target_port"]) != "8080" {
		t.Errorf("target fields not populated: %s", data)
	}
}

// markdownSection returns the body of the "## name" section of a report.
func markdownSection(t *testing.T, text, name string) string {
	t.Helper()
	heading := "\n## " + name + "\n"
	start := strings.Index(text, heading)
	if start < 0 {
		t.Fatalf("report has no %q section:\n%s", name, text)
	}
	body := text[start+len(heading):]
	if end := strings.Index(body, "\n## "); end >= 0 {
		body = body[:end]
	}
	return body
}

func TestBuildDefaultsTitleAndClampsNegativeTop(t *testing.T) {
	got := Build(sampleSession(), Options{Top: -1})
	if got.Title != "DevPulse Traffic Report" || len(got.Slowest) != 0 {
		t.Fatalf("unexpected defaults: title=%q slowest=%d", got.Title, len(got.Slowest))
	}
}
