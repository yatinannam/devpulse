package anomaly

import (
    "encoding/json"
    "testing"
)

func TestFindingJSONIncludesCoreFields(t *testing.T) {
    finding := Finding{Severity: SeverityWarning, Kind: "latency-regression", Subject: "GET /items/:id", Message: "latency increased"}
    data, err := json.Marshal(finding)
    if err != nil {
        t.Fatalf("marshal finding: %v", err)
    }
    var decoded map[string]any
    if err := json.Unmarshal(data, &decoded); err != nil {
        t.Fatalf("unmarshal finding: %v", err)
    }
    for _, key := range []string{"Severity", "Kind", "Subject", "Message"} {
        if _, ok := decoded[key]; !ok {
            t.Fatalf("missing JSON field %q in %s", key, data)
        }
    }
}
