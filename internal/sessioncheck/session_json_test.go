package sessioncheck

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReportHasStableJSONFields(t *testing.T) {
	report := Validate(validSession())
	data, err := json.Marshal(report)
	if err != nil { t.Fatal(err) }
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil { t.Fatal(err) }
	for _, key := range []string{"valid", "request_count", "issues"} {
		if _, ok := fields[key]; !ok { t.Errorf("missing JSON field %q: %s", key, data) }
	}
	if len(report.Issues) != 0 { t.Fatalf("empty session should be valid: %+v", report) }
	_ = time.Second
}
